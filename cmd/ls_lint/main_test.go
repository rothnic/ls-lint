package main

import (
	"reflect"
	"sync"
	"testing"

	"github.com/loeffel-io/ls-lint/v2/internal/rule"
)

func TestGetRuleMessages(t *testing.T) {
	tests := []struct {
		description    string
		ruleErr        *rule.Error
		contextMessage string
		expected       []string
	}{
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
	}

	for _, test := range tests {
		messages := getRuleMessages(test.ruleErr, test.contextMessage)
		if !reflect.DeepEqual(messages, test.expected) {
			t.Fatalf("%s: expected %+v, got %+v", test.description, test.expected, messages)
		}
	}
}
