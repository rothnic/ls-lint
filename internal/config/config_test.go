package config

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/loeffel-io/ls-lint/v2/internal/rule"
	"go.yaml.in/yaml/v3"
)

func TestGetConfig(t *testing.T) {
	config := new(Config)
	indexMock := map[string]map[string][]rule.Rule{
		".": {
			".dir": []rule.Rule{rule.RulesIndex["lowercase"]},
		},
		"./src": {
			".dir": []rule.Rule{rule.RulesIndex["camelcase"]},
		},
	}
	indexMockEmpty := make(RuleIndex)

	tests := []*struct {
		config   *Config
		index    RuleIndex
		path     string
		expected map[string][]rule.Rule
	}{
		{
			config: config,
			index:  indexMock,
			path:   "./src/test/Test.js",
			expected: map[string][]rule.Rule{
				".dir": {rule.RulesIndex["camelcase"]},
			},
		},
		{
			config: config,
			index:  indexMock,
			path:   "./images/path.png",
			expected: map[string][]rule.Rule{
				".dir": {rule.RulesIndex["lowercase"]},
			},
		},
		{
			config:   config,
			index:    indexMockEmpty,
			path:     "./images/path.png",
			expected: nil,
		},
	}

	i := 0
	for _, test := range tests {
		_, res := test.config.GetConfig(test.index, test.path)

		if !reflect.DeepEqual(res, test.expected) {
			t.Errorf("Test %d failed with unmatched return value - %+v", i, res)
			return
		}

		i++
	}
}

func TestGetIgnoreIndex(t *testing.T) {
	tests := []struct {
		description   string
		config        *Config
		expectedExact map[string]bool
		expectedGlob  []string
		expectedErr   string
	}{
		{
			description: "splits exact and glob ignores",
			config: NewConfig(nil, []string{
				"node_modules",
				".env*",
				"packages/*/dist",
				`literal\*name`,
			}),
			expectedExact: map[string]bool{
				"node_modules":  true,
				`literal\*name`: true,
			},
			expectedGlob: []string{
				".env*",
				"packages/*/dist",
			},
		},
		{
			description: "fails for invalid glob ignore",
			config: NewConfig(nil, []string{
				"[",
			}),
			expectedErr: `invalid ignore pattern "["`,
		},
	}

	for _, test := range tests {
		index, err := test.config.GetIgnoreIndex()
		if test.expectedErr != "" {
			if err == nil || !errors.Is(err, ErrInvalidIgnorePattern) {
				t.Fatalf("%s: expected invalid ignore pattern error %q, got %v", test.description, test.expectedErr, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: expected no error, got %v", test.description, err)
		}
		if !reflect.DeepEqual(index.Exact, test.expectedExact) {
			t.Fatalf("%s: expected exact index %+v, got %+v", test.description, test.expectedExact, index.Exact)
		}
		if !reflect.DeepEqual(index.Glob, test.expectedGlob) {
			t.Fatalf("%s: expected glob index %+v, got %+v", test.description, test.expectedGlob, index.Glob)
		}
	}
}

func TestGetContextMessage(t *testing.T) {
	tests := []struct {
		description     string
		config          *Config
		context         string
		expectedMessage string
		expectedFound   bool
	}{
		{
			description: "returns selected context message",
			config: func() *Config {
				config := NewConfig(nil, nil)
				config.Contexts = map[string]Context{
					"pre-commit": {
						Message: "Pre-commit failures are warnings about project shape and naming requirements.",
					},
				}

				return config
			}(),
			context:         "pre-commit",
			expectedMessage: "Pre-commit failures are warnings about project shape and naming requirements.",
			expectedFound:   true,
		},
		{
			description: "formats structured context policy details",
			config: func() *Config {
				config := NewConfig(nil, nil)
				config.Contexts = map[string]Context{
					"pre-push": {
						Message:        "Resolve these failures before pushing or get explicit approval.",
						Mode:           ContextModeFail,
						Hook:           "pre-push",
						Environment:    "local",
						Override:       "repository owner approval",
						ChangeApproval: "repository owner approval",
						References:     []string{"docs/reference/context-policies.md"},
					},
				}

				return config
			}(),
			context:         "pre-push",
			expectedMessage: "Context `pre-push`: blocking, pre-push hook, local environment. Override approval: repository owner approval. Change approval: repository owner approval. References: docs/reference/context-policies.md. Resolve these failures before pushing or get explicit approval.",
			expectedFound:   true,
		},
		{
			description:   "returns false when context is missing",
			config:        NewConfig(Ls{".png": "snake_case"}, nil),
			context:       "pre-push",
			expectedFound: false,
		},
	}

	for _, test := range tests {
		message, found := test.config.GetContextMessage(test.context)
		if found != test.expectedFound {
			t.Fatalf("%s: expected found=%t, got %t", test.description, test.expectedFound, found)
		}
		if message != test.expectedMessage {
			t.Fatalf("%s: expected message %q, got %q", test.description, test.expectedMessage, message)
		}
	}
}

func TestContext_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		description   string
		content       string
		expected      Context
		expectedError string
	}{
		{
			description: "supports scalar context values",
			content: `
contexts:
  pre-commit: Commit is not blocked. Treat these failures as warnings.
`,
			expected: Context{
				Message: "Commit is not blocked. Treat these failures as warnings.",
			},
		},
		{
			description: "supports structured context values",
			content: `
contexts:
  pre-commit:
    mode: warn
    hook: pre-commit
    environment: local
    override: repository owner approval
    change-approval: repository owner approval
    references:
      - docs/contributing.md
    message: >
      Treat these failures as early warnings.
`,
			expected: Context{
				Message:        "Treat these failures as early warnings.\n",
				Mode:           ContextModeWarn,
				Hook:           "pre-commit",
				Environment:    "local",
				Override:       "repository owner approval",
				ChangeApproval: "repository owner approval",
				References:     []string{"docs/contributing.md"},
			},
		},
		{
			description: "allows empty structured mode",
			content: `
contexts:
  pre-commit:
    mode: ""
    message: Context without explicit mode.
`,
			expected: Context{
				Message: "Context without explicit mode.",
			},
		},
		{
			description: "rejects invalid structured mode",
			content: `
contexts:
  pre-commit:
    mode: maybe
`,
			expectedError: `context mode "maybe" is invalid, expected "warn" or "fail"`,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			config := NewConfig(nil, nil)
			err := yaml.Unmarshal([]byte(test.content), config)
			if test.expectedError != "" {
				if err == nil || err.Error() != test.expectedError {
					t.Fatalf("expected error %q, got %v", test.expectedError, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			context, found := config.GetContext("pre-commit")
			if !found {
				t.Fatalf("expected context to be found")
			}
			if context.Message != test.expected.Message {
				t.Fatalf("expected message %q, got %q", test.expected.Message, context.Message)
			}
			if context.Mode != test.expected.Mode {
				t.Fatalf("expected mode %q, got %q", test.expected.Mode, context.Mode)
			}
			if context.Hook != test.expected.Hook {
				t.Fatalf("expected hook %q, got %q", test.expected.Hook, context.Hook)
			}
			if context.Environment != test.expected.Environment {
				t.Fatalf("expected environment %q, got %q", test.expected.Environment, context.Environment)
			}
			if context.Override != test.expected.Override {
				t.Fatalf("expected override %q, got %q", test.expected.Override, context.Override)
			}
			if context.ChangeApproval != test.expected.ChangeApproval {
				t.Fatalf("expected change approval %q, got %q", test.expected.ChangeApproval, context.ChangeApproval)
			}
			if !reflect.DeepEqual(context.References, test.expected.References) {
				t.Fatalf("expected references %+v, got %+v", test.expected.References, context.References)
			}
		})
	}
}

func TestDocumentedHookContextBehavior(t *testing.T) {
	lslintConfig := NewConfig(nil, nil)
	content := []byte(`
contexts:
  pre-commit:
    mode: warn
    hook: pre-commit
    environment: local
    override: repository owner approval
    change-approval: repository owner approval
    references:
      - docs/reference/context-policies.md
    message: >
      Treat these failures as early warnings about repository structure and
      naming so they can be fixed before push.
`)

	if err := yaml.Unmarshal(content, lslintConfig); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	context, found := lslintConfig.GetContext("pre-commit")
	if !found {
		t.Fatalf("expected pre-commit context to be found")
	}
	if !context.ShouldWarn() {
		t.Fatalf("expected pre-commit context to resolve to warn mode")
	}

	message, found := lslintConfig.GetContextMessage("pre-commit")
	if !found {
		t.Fatalf("expected pre-commit context message to be found")
	}
	expected := strings.Join([]string{
		"Context `pre-commit`: warning, pre-commit hook, local environment.",
		"Override approval: repository owner approval.",
		"Change approval: repository owner approval.",
		"References: docs/reference/context-policies.md.",
		"Treat these failures as early warnings about repository structure and naming so they can be fixed before push.",
	}, " ")
	if message != expected {
		t.Fatalf("expected message %q, got %q", expected, message)
	}
}

func TestGetIndex_CustomRuleFeedback(t *testing.T) {
	lslintConfig := NewConfig(Ls{
		".png": "snake_case => PNG files must use snake_case",
		".md":  "regex:^(README|AGENTS)$ => Markdown files must be README.md or AGENTS.md",
		".go":  "camelCase",
	}, nil)

	index, err := lslintConfig.GetIndex(lslintConfig.GetLs())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	pngRules := index[""][".png"]
	if len(pngRules) != 1 {
		t.Fatalf("expected 1 png rule, got %d", len(pngRules))
	}
	if pngRules[0].GetName() != new(rule.SnakeCase).Init().GetName() {
		t.Fatalf("expected snake_case rule, got %q", pngRules[0].GetName())
	}
	if pngRules[0].GetErrorMessage() != "PNG files must use snake_case" {
		t.Fatalf("expected custom png message, got %q", pngRules[0].GetErrorMessage())
	}

	mdRules := index[""][".md"]
	if len(mdRules) != 1 {
		t.Fatalf("expected 1 md rule, got %d", len(mdRules))
	}
	if !reflect.DeepEqual(mdRules[0].GetParameters(), []string{"^(README|AGENTS)$"}) {
		t.Fatalf("expected regex parameters to be preserved, got %+v", mdRules[0].GetParameters())
	}
	if mdRules[0].GetErrorMessage() != "Markdown files must be README.md or AGENTS.md" {
		t.Fatalf("expected custom md message, got %q", mdRules[0].GetErrorMessage())
	}

	goRules := index[""][".go"]
	if len(goRules) != 1 {
		t.Fatalf("expected 1 go rule, got %d", len(goRules))
	}
	if goRules[0].GetErrorMessage() != new(rule.CamelCase).Init().GetErrorMessage() {
		t.Fatalf("expected default rule error message, got %q", goRules[0].GetErrorMessage())
	}
}

func TestShouldIgnore(t *testing.T) {
	tests := []struct {
		lslintConfig *Config
		ignoreIndex  *IgnoreIndex
		path         string
		expected     bool
	}{
		{
			lslintConfig: NewConfig(nil, nil),
			ignoreIndex: &IgnoreIndex{
				Exact: map[string]bool{
					".git": true,
				},
			},
			path:     ".git",
			expected: true,
		},
		{
			lslintConfig: NewConfig(nil, nil),
			ignoreIndex: &IgnoreIndex{
				Exact: map[string]bool{
					"src": true,
				},
			},
			path:     "src/test/test.js",
			expected: true,
		},
		{
			lslintConfig: NewConfig(nil, nil),
			ignoreIndex: &IgnoreIndex{
				Exact: map[string]bool{},
				Glob:  []string{".env*"},
			},
			path:     ".env.local",
			expected: true,
		},
		{
			lslintConfig: NewConfig(nil, nil),
			ignoreIndex: &IgnoreIndex{
				Exact: map[string]bool{},
				Glob:  []string{"**/.env*"},
			},
			path:     "packages/ui/.env.local",
			expected: true,
		},
		{
			lslintConfig: NewConfig(nil, nil),
			ignoreIndex: &IgnoreIndex{
				Exact: map[string]bool{},
				Glob:  []string{"packages/*/dist"},
			},
			path:     "packages/ui/dist/index.js",
			expected: true,
		},
	}

	i := 0
	for _, test := range tests {
		res := test.lslintConfig.ShouldIgnore(test.ignoreIndex, test.path)

		if res != test.expected {
			t.Errorf("Test %d failed with unmatched return value - %+v", i, res)
			return
		}

		i++
	}
}
