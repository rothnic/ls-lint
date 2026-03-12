package config

import (
	"errors"
	"reflect"
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

func TestConfigYAMLAnchorsForSharedRuleSets(t *testing.T) {
	configYAML := []byte(`
shared:
  jsTsDefault: &js_ts_default camelCase | PascalCase | content:max-lines:4
  jsTsRelaxed: &js_ts_relaxed camelCase | PascalCase
ls:
  .js: *js_ts_default
  .ts: *js_ts_default
  vendor:
    .js: *js_ts_relaxed
    .ts: *js_ts_relaxed
ignore:
  - node_modules
`)

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
