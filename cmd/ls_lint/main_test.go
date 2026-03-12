package main

import (
	"reflect"
	"sync"
	"testing"

	"github.com/loeffel-io/ls-lint/v2/internal/rule"
)

func TestGetRuleMessages(t *testing.T) {
	configuredExists := func(t *testing.T) rule.Rule {
		t.Helper()

		existsRule := new(rule.Exists).Init()
		if err := existsRule.SetParameters([]string{}); err != nil {
			t.Fatalf("configure exists rule: %v", err)
		}

		return existsRule
	}

	tests := []struct {
		description    string
		ruleErr        *rule.Error
		contextMessage string
		expected       []string
	}{
		{
			description: "returns base rule message without context",
			ruleErr: &rule.Error{
				Path: "not-snake-case.png",
				Ext:  ".png",
				Rules: []rule.Rule{
					new(rule.SnakeCase).Init(),
				},
				RWMutex: new(sync.RWMutex),
			},
			expected: []string{
				"snakecase",
			},
		},
		{
			description: "prepends context message before rule feedback",
			ruleErr: &rule.Error{
				Path: "not-snake-case.png",
				Ext:  ".png",
				Rules: []rule.Rule{
					rule.NewFeedback(new(rule.SnakeCase).Init(), "PNG files must use snake_case"),
				},
				RWMutex: new(sync.RWMutex),
			},
			contextMessage: "Pre-commit failures are warnings about project shape and naming requirements.",
			expected: []string{
				"Pre-commit failures are warnings about project shape and naming requirements.",
				"PNG files must use snake_case",
			},
		},
		{
			description: "skips file exists rules without leaving a dangling context message",
			ruleErr: &rule.Error{
				Path: "README.md",
				Ext:  "README.md",
				Rules: []rule.Rule{
					new(rule.Exists).Init(),
				},
				RWMutex: new(sync.RWMutex),
			},
			contextMessage: "Pre-push failures block the push until they are resolved.",
			expected:       []string{},
		},
		{
			description: "keeps exists rules for directories and prepends context once",
			ruleErr: &rule.Error{
				Path: "packages/example",
				Dir:  true,
				Ext:  "README.md",
				Rules: []rule.Rule{
					configuredExists(t),
					rule.NewFeedback(new(rule.Regex).Init(), "README must include the package heading"),
				},
				RWMutex: new(sync.RWMutex),
			},
			contextMessage: "Pre-merge failures must be resolved before merging.",
			expected: []string{
				"Pre-merge failures must be resolved before merging.",
				"exists:1-32767 (found 0)",
				"README must include the package heading",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			messages := getRuleMessages(test.ruleErr, test.contextMessage)
			if !reflect.DeepEqual(messages, test.expected) {
				t.Fatalf("expected %+v, got %+v", test.expected, messages)
			}
		})
	}
}
