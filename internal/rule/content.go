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
	lines := contentLines(content)

	rule.Lock()
	defer rule.Unlock()

	rule.result = ""

	switch rule.kind {
	case contentRuleMaxLines:
		count := len(lines)
		if count <= rule.max {
			return true, nil
		}

		rule.result = fmt.Sprintf(" (found %d)", count)
		return false, nil
	case contentRuleMaxLineLength:
		maxLength := 0
		for _, line := range lines {
			lineLength := utf8.RuneCountInString(line)
			if lineLength > maxLength {
				maxLength = lineLength
			}
		}

		if maxLength <= rule.max {
			return true, nil
		}

		rule.result = fmt.Sprintf(" (found %d)", maxLength)
		return false, nil
	case contentRuleHeading:
		for _, line := range lines {
			if rule.regex.MatchString(line) {
				return true, nil
			}
		}

		return false, nil
	case contentRuleFrontMatter:
		return hasFrontMatter(lines), nil
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

func hasFrontMatter(lines []string) bool {
	if len(lines) < 2 || lines[0] != "---" {
		return false
	}

	for _, line := range lines[1:] {
		if line == "---" || line == "..." {
			return true
		}
	}

	return false
}
