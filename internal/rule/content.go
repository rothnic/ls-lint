package rule

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	contentRuleMaxLines       = "max-lines"
	contentRuleMaxLineLength  = "max-line-length"
	contentRuleHeading        = "heading"
	contentRuleFrontMatter    = "front-matter"
	contentRuleFrontMatterReq = "required"
)

type Content struct {
	name          string
	exclusive     bool
	kind          string
	rawParameters string
	max           int
	pattern       string
	regex         *regexp.Regexp
	result        string
	*sync.RWMutex
}

type PreparedContent struct {
	lines          []string
	lineCount      int
	maxLineLength  int
	hasFrontMatter bool
}

type PreparedContentOptions struct {
	MaxLineLength bool
	FrontMatter   bool
}

func (rule *Content) Init() Rule {
	rule.name = "content"
	rule.exclusive = false
	rule.RWMutex = new(sync.RWMutex)

	return rule
}

func (rule *Content) GetName() string {
	rule.RLock()
	defer rule.RUnlock()

	return rule.name
}

func (rule *Content) SetParameters(params []string) error {
	rule.Lock()
	defer rule.Unlock()

	if len(params) == 0 || params[0] == "" {
		return fmt.Errorf("content rule not exists")
	}

	rule.rawParameters = params[0]
	rule.result = ""

	parts := strings.SplitN(params[0], ":", 2)
	rule.kind = parts[0]

	value := ""
	if len(parts) == 2 {
		value = parts[1]
	}

	switch rule.kind {
	case contentRuleMaxLines, contentRuleMaxLineLength:
		if value == "" {
			return fmt.Errorf("%s value is empty", rule.kind)
		}

		max, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		if max < 0 {
			return fmt.Errorf("%s value must be greater or equal to 0", rule.kind)
		}

		rule.max = max
		rule.pattern = ""
		rule.regex = nil
		return nil
	case contentRuleHeading:
		if value == "" {
			return fmt.Errorf("heading pattern is empty")
		}

		regex, err := regexp.Compile(value)
		if err != nil {
			return err
		}

		rule.pattern = value
		rule.regex = regex
		rule.max = 0
		return nil
	case contentRuleFrontMatter:
		if value != contentRuleFrontMatterReq {
			return fmt.Errorf("front-matter value must be %s", contentRuleFrontMatterReq)
		}

		rule.pattern = value
		rule.regex = nil
		rule.max = 0
		return nil
	default:
		return fmt.Errorf("unknown content rule %s", rule.kind)
	}
}

func (rule *Content) GetParameters() []string {
	rule.RLock()
	defer rule.RUnlock()

	if rule.rawParameters == "" {
		return nil
	}

	return []string{rule.rawParameters}
}

func (rule *Content) GetExclusive() bool {
	rule.RLock()
	defer rule.RUnlock()

	return rule.exclusive
}

func (rule *Content) Validate(_ string, _ string, _ bool) (bool, error) {
	return true, nil
}

func (rule *Content) ValidateContent(content []byte, _ string) (bool, error) {
	return rule.ValidatePreparedContent(NewPreparedContent(content, rule.GetPreparedContentOptions()), "")
}

func (rule *Content) ValidatePreparedContent(content *PreparedContent, _ string) (bool, error) {
	if content == nil {
		content = &PreparedContent{}
	}

	rule.Lock()
	defer rule.Unlock()

	rule.result = ""

	switch rule.kind {
	case contentRuleMaxLines:
		if content.lineCount <= rule.max {
			return true, nil
		}

		rule.result = fmt.Sprintf(" (found %d)", content.lineCount)
		return false, nil
	case contentRuleMaxLineLength:
		if content.maxLineLength <= rule.max {
			return true, nil
		}

		rule.result = fmt.Sprintf(" (found %d)", content.maxLineLength)
		return false, nil
	case contentRuleHeading:
		for _, line := range content.lines {
			if rule.regex.MatchString(line) {
				return true, nil
			}
		}

		return false, nil
	case contentRuleFrontMatter:
		return content.hasFrontMatter, nil
	default:
		return false, fmt.Errorf("unknown content rule %s", rule.kind)
	}
}

func (rule *Content) GetErrorMessage() string {
	rule.RLock()
	defer rule.RUnlock()

	message := fmt.Sprintf("%s:%s", rule.name, rule.rawParameters)
	if rule.result == "" {
		return message
	}

	return message + rule.result
}

func (rule *Content) Copy() Rule {
	rule.RLock()
	defer rule.RUnlock()

	c := new(Content)
	c.Init()
	c.kind = rule.kind
	c.rawParameters = rule.rawParameters
	c.max = rule.max
	c.pattern = rule.pattern
	c.regex = rule.regex
	c.result = rule.result
	return c
}

func contentLines(content []byte) []string {
	normalized := strings.ReplaceAll(string(content), "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	normalized = strings.TrimPrefix(normalized, "\ufeff")
	if normalized == "" {
		return nil
	}

	lines := strings.Split(normalized, "\n")
	if strings.HasSuffix(normalized, "\n") {
		lines = lines[:len(lines)-1]
	}

	return lines
}

func NewPreparedContent(content []byte, options PreparedContentOptions) *PreparedContent {
	lines := contentLines(content)

	prepared := &PreparedContent{
		lines:          lines,
		lineCount:      len(lines),
		maxLineLength:  0,
		hasFrontMatter: false,
	}

	if options.MaxLineLength {
		for _, line := range lines {
			lineLength := utf8.RuneCountInString(line)
			if lineLength > prepared.maxLineLength {
				prepared.maxLineLength = lineLength
			}
		}
	}

	if options.FrontMatter {
		prepared.hasFrontMatter = hasFrontMatter(lines)
	}

	return prepared
}

func (rule *Content) GetPreparedContentOptions() PreparedContentOptions {
	rule.RLock()
	defer rule.RUnlock()

	return PreparedContentOptions{
		MaxLineLength: rule.kind == contentRuleMaxLineLength,
		FrontMatter:   rule.kind == contentRuleFrontMatter,
	}
}

func hasFrontMatter(lines []string) bool {
	if len(lines) < 3 || lines[0] != "---" {
		return false
	}

	hasContent := false
	for _, line := range lines[1:] {
		if line == "---" || line == "..." {
			return hasContent
		}
		if strings.TrimSpace(line) != "" {
			hasContent = true
		}
	}

	return false
}
