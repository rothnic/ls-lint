package config

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/loeffel-io/ls-lint/v2/internal/rule"
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
	sep = string('/')
	or  = " | "
)

var ErrInvalidIgnorePattern = errors.New("invalid ignore pattern")

type Config struct {
	Ls         Ls                     `yaml:"ls"`
	RuleGroups map[string]interface{} `yaml:"rule-groups"`
	Ignore     []string               `yaml:"ignore"`
	*sync.RWMutex
}

func NewConfig(ls Ls, ignore []string) *Config {
	return &Config{
		Ls:         ls,
		RuleGroups: make(map[string]interface{}),
		Ignore:     ignore,
		RWMutex:    new(sync.RWMutex),
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

	return config.RuleGroups
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

		ruleNames, err := config.expandRuleNames(strings.Split(v.(string), or), nil)
		if err != nil {
			return err
		}

		for _, ruleName := range ruleNames {
			ruleName = strings.TrimSpace(ruleName)
			ruleSplit := strings.SplitN(ruleName, ":", 2)
			ruleName = ruleSplit[0]

			if r, ok := rule.Rules[ruleName]; ok {
				r = r.Copy()

				if err := r.SetParameters(ruleSplit[1:]); err != nil {
					return fmt.Errorf("rule %s failed with %s", ruleName, err.Error())
				}

				index[key][k] = append(index[key][k], r)
				continue
			}

			return fmt.Errorf("rule %s not exists", ruleName)
		}
	}

	return nil
}

func (config *Config) expandRuleNames(ruleNames []string, seenGroups map[string]bool) ([]string, error) {
	expanded := make([]string, 0, len(ruleNames))

	for _, ruleName := range ruleNames {
		ruleName = strings.TrimSpace(ruleName)
		if ruleName == "" {
			continue
		}

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

	return expanded, nil
}

func (config *Config) getRuleGroupRules(name string, seenGroups map[string]bool) ([]string, error) {
	if name == "" {
		return nil, fmt.Errorf("rule group name empty")
	}
	if seenGroups != nil && seenGroups[name] {
		return nil, fmt.Errorf("rule group %s circular reference", name)
	}

	groupValue, exists := config.GetRuleGroups()[name]
	if !exists {
		return nil, fmt.Errorf("rule group %s not exists", name)
	}

	groupEntries, err := normalizeRuleGroupEntries(groupValue)
	if err != nil {
		return nil, fmt.Errorf("rule group %s failed with %s", name, err.Error())
	}

	nextSeenGroups := make(map[string]bool, len(seenGroups)+1)
	for key, value := range seenGroups {
		nextSeenGroups[key] = value
	}
	nextSeenGroups[name] = true

	return config.expandRuleNames(groupEntries, nextSeenGroups)
}

func normalizeRuleGroupEntries(groupValue interface{}) ([]string, error) {
	switch reflect.TypeOf(groupValue).Kind() {
	case reflect.String:
		return strings.Split(groupValue.(string), or), nil
	case reflect.Slice:
		groupSlice := reflect.ValueOf(groupValue)
		entries := make([]string, 0, groupSlice.Len())
		for i := 0; i < groupSlice.Len(); i++ {
			entry, ok := groupSlice.Index(i).Interface().(string)
			if !ok {
				return nil, fmt.Errorf("rule group entry must be a string")
			}
			entries = append(entries, entry)
		}
		return entries, nil
	default:
		return nil, fmt.Errorf("rule group must be a string or list of strings")
	}
}
