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

func TestGetIndex_ContentRules(t *testing.T) {
	config := NewConfig(Ls{
		".md": "kebab-case | content:max-lines:10 | content:heading:^## Overview$ | content:front-matter:required",
	}, nil)

	index, err := config.GetIndex(config.GetLs())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	rules := index[""][".md"]
	expected := []struct {
		name   string
		params []string
	}{
		{name: "kebabcase", params: nil},
		{name: "content", params: []string{"max-lines:10"}},
		{name: "content", params: []string{"heading:^## Overview$"}},
		{name: "content", params: []string{"front-matter:required"}},
	}

	if len(rules) != len(expected) {
		t.Fatalf("expected %d rules, got %d", len(expected), len(rules))
	}

	for i, expectedRule := range expected {
		if rules[i].GetName() != expectedRule.name {
			t.Fatalf("expected rule %d name %q, got %q", i, expectedRule.name, rules[i].GetName())
		}
		if !reflect.DeepEqual(rules[i].GetParameters(), expectedRule.params) {
			t.Fatalf("expected rule %d params %v, got %v", i, expectedRule.params, rules[i].GetParameters())
		}
	}
}

func TestGetIndex_InvalidContentRule(t *testing.T) {
	config := NewConfig(Ls{
		".md": "content:not-a-rule:1",
	}, nil)

	_, err := config.GetIndex(config.GetLs())
	if err == nil || err.Error() != "rule content failed with unknown content rule not-a-rule" {
		t.Fatalf("expected invalid content rule error, got %v", err)
	}
}

func TestConfigYAMLRuleGroupsForSharedRuleSets(t *testing.T) {
	configYAML := []byte(
		"\ngroups:\n" +
			"  jsTsDefault:\n" +
			"    - camelCase\n" +
			"    - PascalCase\n" +
			"    - content:max-lines:4\n" +
			"  jsTsRelaxed:\n" +
			"    - camelCase\n" +
			"    - PascalCase\n" +
			"  jsTsNames: \"camelCase | PascalCase\"\n" +
			"ls:\n" +
			"  .js: group:jsTsDefault\n" +
			"  .ts: \"@jsTsDefault\"\n" +
			"  vendor:\n" +
			"    .js: \"@jsTsRelaxed\"\n" +
			"    .ts: \"@jsTsRelaxed\"\n" +
			"ignore:\n" +
			"  - node_modules\n",
	)

	config := NewConfig(nil, nil)
	if err := yaml.Unmarshal(configYAML, config); err != nil {
		t.Fatalf("expected yaml to unmarshal, got %v", err)
	}

	index, err := config.GetIndex(config.GetLs())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	rootTSRules := index[""][".ts"]
	if len(rootTSRules) != 3 {
		t.Fatalf("expected 3 root .ts rules, got %d", len(rootTSRules))
	}
	if rootTSRules[2].GetName() != "content" || !reflect.DeepEqual(rootTSRules[2].GetParameters(), []string{"max-lines:4"}) {
		t.Fatalf("expected root .ts to include max-lines content rule, got %s %v", rootTSRules[2].GetName(), rootTSRules[2].GetParameters())
	}

	vendorTSRules := index["vendor"][".ts"]
	if len(vendorTSRules) != 2 {
		t.Fatalf("expected 2 vendor .ts rules, got %d", len(vendorTSRules))
	}
	for _, ruleFile := range vendorTSRules {
		if ruleFile.GetName() == "content" {
			t.Fatalf("expected vendor .ts override to omit content rules, got %v", vendorTSRules)
		}
	}

	if !reflect.DeepEqual(config.GetIgnore(), []string{"node_modules"}) {
		t.Fatalf("expected ignore list to survive yaml unmarshal, got %v", config.GetIgnore())
	}
}

func TestConfigYAMLGroupAliasAndNestedReuse(t *testing.T) {
	configYAML := []byte(
		"\ngroups:\n" +
			"  shared-js: \"camelCase | PascalCase\"\n" +
			"  js-defaults:\n" +
			"    - \"@shared-js\"\n" +
			"    - content:max-lines:4\n" +
			"  js-long-form:\n" +
			"    - \"@shared-js\"\n" +
			"    - content:max-lines:800\n" +
			"ls:\n" +
			"  .ts: \"@js-defaults\"\n" +
			"  generated:\n" +
			"    .ts: \"@js-long-form\"\n",
	)

	config := NewConfig(nil, nil)
	if err := yaml.Unmarshal(configYAML, config); err != nil {
		t.Fatalf("expected yaml to unmarshal, got %v", err)
	}

	index, err := config.GetIndex(config.GetLs())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	rootTSRules := index[""][".ts"]
	if len(rootTSRules) != 3 {
		t.Fatalf("expected 3 rules for .ts, got %d", len(rootTSRules))
	}
	if rootTSRules[2].GetName() != "content" || !reflect.DeepEqual(rootTSRules[2].GetParameters(), []string{"max-lines:4"}) {
		t.Fatalf("expected content max-lines:4 rule, got %s %v", rootTSRules[2].GetName(), rootTSRules[2].GetParameters())
	}

	genTSRules := index["generated"][".ts"]
	if len(genTSRules) != 3 {
		t.Fatalf("expected 3 generated .ts rules, got %d", len(genTSRules))
	}
	if genTSRules[2].GetName() != "content" || !reflect.DeepEqual(genTSRules[2].GetParameters(), []string{"max-lines:800"}) {
		t.Fatalf("expected content max-lines:800 rule, got %s %v", genTSRules[2].GetName(), genTSRules[2].GetParameters())
	}
}

func TestGetIndex_StringRuleGroup(t *testing.T) {
	config := NewConfig(Ls{
		".ts": "group:jsTsDefault",
	}, nil)
	config.Groups["jsTsDefault"] = "camelCase | content:max-lines:4"

	index, err := config.GetIndex(config.GetLs())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	rules := index[""][".ts"]
	if len(rules) != 2 {
		t.Fatalf("expected 2 root .ts rules, got %d", len(rules))
	}
	if rules[0].GetName() != "camelcase" {
		t.Fatalf("expected first rule name camelcase, got %q", rules[0].GetName())
	}
	if rules[1].GetName() != "content" || !reflect.DeepEqual(rules[1].GetParameters(), []string{"max-lines:4"}) {
		t.Fatalf("expected content max-lines rule, got %s %v", rules[1].GetName(), rules[1].GetParameters())
	}
}

func TestGetIndex_SliceRuleGroup(t *testing.T) {
	config := NewConfig(Ls{
		".ts": "group:jsTsDefault",
	}, nil)
	config.Groups["jsTsDefault"] = []string{"camelCase", "content:max-lines:4"}

	index, err := config.GetIndex(config.GetLs())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	rules := index[""][".ts"]
	if len(rules) != 2 {
		t.Fatalf("expected 2 root .ts rules, got %d", len(rules))
	}
	if rules[0].GetName() != "camelcase" {
		t.Fatalf("expected first rule name camelcase, got %q", rules[0].GetName())
	}
	if rules[1].GetName() != "content" || !reflect.DeepEqual(rules[1].GetParameters(), []string{"max-lines:4"}) {
		t.Fatalf("expected content max-lines rule, got %s %v", rules[1].GetName(), rules[1].GetParameters())
	}
}

func TestGetIndex_MissingRuleGroup(t *testing.T) {
	config := NewConfig(Ls{
		".ts": "group:missing",
	}, nil)

	_, err := config.GetIndex(config.GetLs())
	if err == nil || err.Error() != `rule group "missing" does not exist` {
		t.Fatalf("expected missing rule group error, got %v", err)
	}
}

func TestGetIndex_EmptyRuleGroupName(t *testing.T) {
	config := NewConfig(Ls{
		".ts": "group:",
	}, nil)

	_, err := config.GetIndex(config.GetLs())
	if err == nil || err.Error() != `rule group name is empty` {
		t.Fatalf("expected empty rule group name error, got %v", err)
	}
}

func TestGetIndex_CircularRuleGroup(t *testing.T) {
	config := NewConfig(Ls{
		".ts": "group:groupA",
	}, nil)
	config.Groups["groupA"] = []string{"group:groupB"}
	config.Groups["groupB"] = []string{"group:groupA"}

	_, err := config.GetIndex(config.GetLs())
	if err == nil || err.Error() != `circular reference detected in rule group "groupA"` {
		t.Fatalf("expected circular rule group error, got %v", err)
	}
}

func TestGetIndex_InvalidRuleGroupValue(t *testing.T) {
	tests := []struct {
		description string
		groupValue  interface{}
		expectedErr string
	}{
		{
			description: "invalid rule group type",
			groupValue:  true,
			expectedErr: `rule group "groupA" must be a string or list of strings, got bool`,
		},
		{
			description: "invalid numeric rule group type",
			groupValue:  42,
			expectedErr: `rule group "groupA" must be a string or list of strings, got int`,
		},
		{
			description: "invalid map rule group type",
			groupValue:  map[string]string{"rule": "camelCase"},
			expectedErr: `rule group "groupA" must be a string or list of strings, got map[string]string`,
		},
		{
			description: "invalid rule group entry type",
			groupValue:  []interface{}{"camelCase", 42},
			expectedErr: `rule group "groupA" entry 1 must be a string, got int`,
		},
	}

	for _, test := range tests {
		config := NewConfig(Ls{
			".ts": "group:groupA",
		}, nil)
		config.Groups["groupA"] = test.groupValue

		_, err := config.GetIndex(config.GetLs())
		if err == nil || err.Error() != test.expectedErr {
			t.Fatalf("%s: expected %q, got %v", test.description, test.expectedErr, err)
		}
	}
}
