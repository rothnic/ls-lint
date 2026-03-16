package config

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/loeffel-io/ls-lint/v2/internal/rule"
	"go.yaml.in/yaml/v3"
)

type (
	Ls          map[string]interface{}
	RuleIndex   map[string]map[string][]rule.Rule
	IgnoreIndex struct {
		Exact map[string]bool
		Glob  []string
	}
)

const (
	sep              = string('/')
	or               = " | "
	customMessageSep = " => "
)

var ErrInvalidIgnorePattern = errors.New("invalid ignore pattern")

const (
	ContextModeWarn = "warn"
	ContextModeFail = "fail"
)

type Config struct {
	Ls       Ls                     `yaml:"ls"`
	Groups   map[string]interface{} `yaml:"groups"`
	Ignore   []string               `yaml:"ignore"`
	Contexts map[string]Context     `yaml:"contexts"`
	*sync.RWMutex
}

type Context struct {
	Message        string   `yaml:"message"`
	Mode           string   `yaml:"mode"`
	Hook           string   `yaml:"hook"`
	Environment    string   `yaml:"environment"`
	Override       string   `yaml:"override"`
	ChangeApproval string   `yaml:"change-approval"`
	References     []string `yaml:"references"`
}

func NewConfig(ls Ls, ignore []string) *Config {
	return &Config{
		Ls:      ls,
		Groups:  make(map[string]interface{}),
		Ignore:  ignore,
		RWMutex: new(sync.RWMutex),
	}
}

func (config *Config) GetLs() Ls {
	config.RLock()
	defer config.RUnlock()

	return config.Ls
}

func (config *Config) GetIgnore() []string {
	config.RLock()
	defer config.RUnlock()

	return config.Ignore
}

func (config *Config) GetRuleGroups() map[string]interface{} {
	config.RLock()
	defer config.RUnlock()

	ruleGroups := make(map[string]interface{}, len(config.Groups))
	for key, value := range config.Groups {
		ruleGroups[key] = cloneRuleGroupValue(value)
	}

	return ruleGroups
}

func (config *Config) MergeRuleGroups(ruleGroups map[string]interface{}) {
	config.Lock()
	defer config.Unlock()

	if config.Groups == nil {
		config.Groups = make(map[string]interface{})
	}

	for key, value := range ruleGroups {
		config.Groups[key] = cloneRuleGroupValue(value)
	}
}

func MergeIgnore(current []string, additional []string) []string {
	current = append(current, additional...)
	slices.Sort(current)

	return slices.Compact(current)
}

func (context *Context) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		var message string
		if err := node.Decode(&message); err != nil {
			return err
		}

		context.Message = message
		return nil
	case yaml.MappingNode:
		type rawContext Context

		var raw rawContext
		if err := node.Decode(&raw); err != nil {
			return err
		}

		mode, err := validateAndNormalizeContextMode(raw.Mode)
		if err != nil {
			return err
		}

		*context = Context(raw)
		context.Mode = mode
		return nil
	default:
		return fmt.Errorf("context must be a string or mapping, got yaml kind %d", node.Kind)
	}
}

func validateAndNormalizeContextMode(mode string) (string, error) {
	normalized := strings.TrimSpace(strings.ToLower(mode))
	switch normalized {
	case "", ContextModeWarn, ContextModeFail:
		return normalized, nil
	default:
		return "", fmt.Errorf("context mode %q is invalid, expected %q or %q", mode, ContextModeWarn, ContextModeFail)
	}
}

func (context Context) ShouldWarn() bool {
	return context.Mode == ContextModeWarn
}

func (context Context) GetMessage(name string) string {
	message := strings.TrimSpace(context.Message)
	sentences := make([]string, 0, 5)
	summaryParts := make([]string, 0, 3)

	switch context.Mode {
	case ContextModeWarn:
		summaryParts = append(summaryParts, "warning")
	case ContextModeFail:
		summaryParts = append(summaryParts, "blocking")
	}

	if hook := strings.TrimSpace(context.Hook); hook != "" {
		summaryParts = append(summaryParts, fmt.Sprintf("%s hook", hook))
	}

	if environment := strings.TrimSpace(context.Environment); environment != "" {
		summaryParts = append(summaryParts, fmt.Sprintf("%s environment", environment))
	}

	if override := strings.TrimSpace(context.Override); override != "" {
		sentences = append(sentences, fmt.Sprintf("Override approval: %s.", override))
	}

	if changeApproval := strings.TrimSpace(context.ChangeApproval); changeApproval != "" {
		sentences = append(sentences, fmt.Sprintf("Change approval: %s.", changeApproval))
	}

	references := make([]string, 0, len(context.References))
	for _, reference := range context.References {
		reference = strings.TrimSpace(reference)
		if reference != "" {
			references = append(references, reference)
		}
	}
	if len(references) > 0 {
		sentences = append(sentences, fmt.Sprintf("References: %s.", strings.Join(references, ", ")))
	}

	if len(summaryParts) == 0 && len(sentences) == 0 {
		return message
	}

	if name == "" {
		name = "context"
	}

	if len(summaryParts) > 0 {
		sentences = append([]string{fmt.Sprintf("Context `%s`: %s.", name, strings.Join(summaryParts, ", "))}, sentences...)
	} else {
		sentences = append([]string{fmt.Sprintf("Context `%s`.", name)}, sentences...)
	}

	if message != "" {
		sentences = append(sentences, message)
	}

	return strings.Join(sentences, " ")
}

func (config *Config) GetContext(name string) (Context, bool) {
	config.RLock()
	defer config.RUnlock()

	if name == "" {
		return Context{}, false
	}

	context, exists := config.Contexts[name]

	return context, exists
}

func (config *Config) GetContextMessage(name string) (string, bool) {
	context, exists := config.GetContext(name)
	if !exists {
		return "", false
	}

	return context.GetMessage(name), true
}

func (config *Config) GetIgnoreIndex() (*IgnoreIndex, error) {
	ignoreIndex := &IgnoreIndex{
		Exact: make(map[string]bool),
		Glob:  make([]string, 0),
	}

	for _, path := range config.GetIgnore() {
		if hasGlobPattern(path) {
			if !doublestar.ValidatePattern(path) {
				return nil, fmt.Errorf("%w %q", ErrInvalidIgnorePattern, path)
			}

			ignoreIndex.Glob = append(ignoreIndex.Glob, path)
			continue
		}

		ignoreIndex.Exact[path] = true
	}

	return ignoreIndex, nil
}

func (config *Config) ShouldIgnore(ignoreIndex *IgnoreIndex, path string) bool {
	if ignoreIndex == nil {
		return false
	}

	for candidate := path; candidate != ""; candidate = getParentPath(candidate) {
		if ignore, exists := ignoreIndex.Exact[candidate]; exists {
			return ignore
		}
	}

	for candidate := path; candidate != ""; candidate = getParentPath(candidate) {
		for _, pattern := range ignoreIndex.Glob {
			if doublestar.MatchUnvalidated(pattern, candidate) {
				return true
			}
		}
	}

	return false
}

// getParentPath returns the parent path by dropping the last slash-delimited
// segment. It returns an empty string when the path has no parent.
func getParentPath(path string) string {
	index := strings.LastIndex(path, sep)
	if index == -1 {
		return ""
	}

	return path[:index]
}

func hasGlobPattern(path string) bool {
	escaped := false
	for _, char := range path {
		if escaped {
			escaped = false
			continue
		}

		if char == '\\' {
			escaped = true
			continue
		}

		switch char {
		case '*', '?', '[', ']', '{', '}':
			return true
		}
	}

	return false
}

func (config *Config) GetConfig(index RuleIndex, path string) (string, map[string][]rule.Rule) {
	dirs := strings.Split(path, sep)

	for i := len(dirs); i >= 0; i-- {
		dir := strings.Join(dirs[:i], sep)
		if find, exists := index[dir]; exists {
			return dir, find
		}
	}

	return "", nil
}

func (config *Config) GetIndex(list Ls) (RuleIndex, error) {
	index := make(RuleIndex)

	if err := config.walkIndex(index, "", list); err != nil {
		return nil, err
	}

	return index, nil
}

func (config *Config) walkIndex(index RuleIndex, key string, list Ls) error {
	if index[key] == nil {
		index[key] = make(map[string][]rule.Rule)
	}

	for k, v := range list {
		if v == nil {
			continue
		}

		if reflect.TypeOf(v).Kind() == reflect.Map {
			switch key == "" {
			case true:
				if err := config.walkIndex(index, k, v.(Ls)); err != nil {
					return err
				}
			case false:
				keyCombination := fmt.Sprintf("%s%s%s", key, sep, k)
				if err := config.walkIndex(index, keyCombination, v.(Ls)); err != nil {
					return err
				}
			}

			continue
		}

		ruleDefinitions, err := config.expandRuleNames(strings.Split(v.(string), or), nil)
		if err != nil {
			return err
		}

		for _, ruleDefinition := range ruleDefinitions {
			ruleName, ruleParameters, message, err := parseRuleDefinition(ruleDefinition)
			if err != nil {
				return err
			}

			if r, ok := rule.Rules[ruleName]; ok {
				r = r.Copy()

				if err := r.SetParameters(ruleParameters); err != nil {
					return fmt.Errorf("rule %s failed with %s", ruleName, err.Error())
				}

				index[key][k] = append(index[key][k], rule.NewFeedback(r, message))
				continue
			}

			return fmt.Errorf("rule %s does not exist", ruleName)
		}
	}

	return nil
}

func (config *Config) expandRuleNames(ruleNames []string, seenGroups map[string]bool) ([]string, error) {
	if seenGroups == nil {
		seenGroups = make(map[string]bool)
	}

	expanded := make([]string, 0, len(ruleNames))

	for _, ruleName := range ruleNames {
		ruleName = strings.TrimSpace(ruleName)
		if ruleName == "" {
			continue
		}

		switch {
		case strings.HasPrefix(ruleName, "@"):
			groupRules, err := config.getRuleGroupRules(strings.TrimSpace(strings.TrimPrefix(ruleName, "@")), seenGroups)
			if err != nil {
				return nil, err
			}
			expanded = append(expanded, groupRules...)
			continue
		default:
			groupName, isRuleGroup := strings.CutPrefix(ruleName, "group:")
			if !isRuleGroup {
				expanded = append(expanded, ruleName)
				continue
			}

			groupName = strings.TrimSpace(groupName)
			groupRules, err := config.getRuleGroupRules(groupName, seenGroups)
			if err != nil {
				return nil, err
			}
			expanded = append(expanded, groupRules...)
		}
	}

	return expanded, nil
}

func (config *Config) getRuleGroupRules(name string, seenGroups map[string]bool) ([]string, error) {
	if name == "" {
		return nil, fmt.Errorf("rule group name is empty")
	}
	if seenGroups != nil && seenGroups[name] {
		return nil, fmt.Errorf("circular reference detected in rule group %q", name)
	}

	groupValue, exists := config.GetRuleGroups()[name]
	if !exists {
		return nil, fmt.Errorf("rule group %q does not exist", name)
	}

	groupEntries, err := normalizeRuleGroupEntries(name, groupValue)
	if err != nil {
		return nil, err
	}

	nextSeenGroups := make(map[string]bool, len(seenGroups)+1)
	maps.Copy(nextSeenGroups, seenGroups)
	nextSeenGroups[name] = true

	return config.expandRuleNames(groupEntries, nextSeenGroups)
}

func cloneRuleGroupValue(groupValue interface{}) interface{} {
	switch value := groupValue.(type) {
	case nil:
		return nil
	case []string:
		return append([]string(nil), value...)
	case []interface{}:
		return append([]interface{}(nil), value...)
	}

	return groupValue
}

func normalizeRuleGroupEntries(name string, groupValue interface{}) ([]string, error) {
	if groupValue == nil {
		return nil, fmt.Errorf("rule group %q must be a string or list of strings, got <nil>", name)
	}

	switch reflect.TypeOf(groupValue).Kind() {
	case reflect.String:
		return strings.Split(groupValue.(string), or), nil
	case reflect.Slice:
		groupSlice := reflect.ValueOf(groupValue)
		entries := make([]string, 0, groupSlice.Len())
		for i := 0; i < groupSlice.Len(); i++ {
			rawEntry := groupSlice.Index(i).Interface()
			entry, ok := rawEntry.(string)
			if !ok {
				return nil, fmt.Errorf("rule group %q entry %d must be a string, got %T", name, i, rawEntry)
			}
			entries = append(entries, entry)
		}
		return entries, nil
	default:
		return nil, fmt.Errorf("rule group %q must be a string or list of strings, got %T", name, groupValue)
	}
}

func parseRuleDefinition(ruleDefinition string) (string, []string, string, error) {
	ruleDefinition = strings.TrimSpace(ruleDefinition)

	message := ""
	if index := strings.LastIndex(ruleDefinition, customMessageSep); index != -1 {
		message = strings.TrimSpace(ruleDefinition[index+len(customMessageSep):])
		if message == "" {
			return "", nil, "", fmt.Errorf("custom message separator found but message text is empty in rule definition %q", ruleDefinition)
		}

		ruleDefinition = strings.TrimSpace(ruleDefinition[:index])
	}

	ruleSplit := strings.SplitN(ruleDefinition, ":", 2)
	ruleName := strings.TrimSpace(ruleSplit[0])
	if ruleName == "" {
		return "", nil, "", fmt.Errorf("rule name is required, got empty string from definition %q", ruleDefinition)
	}

	return ruleName, ruleSplit[1:], message, nil
}
