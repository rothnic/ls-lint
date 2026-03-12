package config

import (
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/loeffel-io/ls-lint/v2/internal/rule"
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

func TestApplyContext(t *testing.T) {
	tests := []struct {
		description     string
		config          *Config
		context         string
		expectedApplied bool
		expectedLs      Ls
		expectedIgnore  []string
	}{
		{
			description: "applies selected context overrides",
			config: &Config{
				Ls: Ls{
					".png": "snake_case => Must use snake_case before merging",
				},
				Ignore: []string{"node_modules"},
				Contexts: map[string]Context{
					"pre-commit": {
						Ls: Ls{
							".png": "snake_case => Prefer snake_case while iterating locally",
							".md":  "kebab-case => Markdown files should stay kebab-case",
						},
						Ignore: []string{"tmp"},
					},
				},
				RWMutex: new(sync.RWMutex),
			},
			context:         "pre-commit",
			expectedApplied: true,
			expectedLs: Ls{
				".png": "snake_case => Prefer snake_case while iterating locally",
				".md":  "kebab-case => Markdown files should stay kebab-case",
			},
			expectedIgnore: []string{"node_modules", "tmp"},
		},
		{
			description: "returns false when context is missing",
			config: &Config{
				Ls: Ls{
					".png": "snake_case",
				},
				Ignore:  []string{"node_modules"},
				RWMutex: new(sync.RWMutex),
			},
			context:         "pre-push",
			expectedApplied: false,
			expectedLs: Ls{
				".png": "snake_case",
			},
			expectedIgnore: []string{"node_modules"},
		},
	}

	for _, test := range tests {
		applied, err := test.config.ApplyContext(test.context)
		if err != nil {
			t.Fatalf("%s: expected no error, got %v", test.description, err)
		}
		if applied != test.expectedApplied {
			t.Fatalf("%s: expected applied=%t, got %t", test.description, test.expectedApplied, applied)
		}
		if !reflect.DeepEqual(test.config.GetLs(), test.expectedLs) {
			t.Fatalf("%s: expected ls %+v, got %+v", test.description, test.expectedLs, test.config.GetLs())
		}
		if !reflect.DeepEqual(test.config.GetIgnore(), test.expectedIgnore) {
			t.Fatalf("%s: expected ignore %+v, got %+v", test.description, test.expectedIgnore, test.config.GetIgnore())
		}
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
