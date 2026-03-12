package rule

import (
	"reflect"
	"testing"
)

func TestContent(t *testing.T) {
	tests := []struct {
		description string
		params      []string
		content     string
		expected    bool
		expectedErr string
	}{
		{
			description: "max lines success",
			params:      []string{"max-lines:2"},
			content:     "first\nsecond\n",
			expected:    true,
		},
		{
			description: "max lines fail",
			params:      []string{"max-lines:1"},
			content:     "first\nsecond\n",
			expected:    false,
		},
		{
			description: "max line length uses rune count",
			params:      []string{"max-line-length:3"},
			content:     "åäö\nok\n",
			expected:    true,
		},
		{
			description: "required heading present",
			params:      []string{"heading:^## Overview$"},
			content:     "# Intro\r\n## Overview\r\n",
			expected:    true,
		},
		{
			description: "required heading missing",
			params:      []string{"heading:^## Overview$"},
			content:     "# Intro\n## Usage\n",
			expected:    false,
		},
		{
			description: "front matter required",
			params:      []string{"front-matter:required"},
			content:     "---\ntitle: Guide\n---\n# Guide\n",
			expected:    true,
		},
		{
			description: "front matter missing",
			params:      []string{"front-matter:required"},
			content:     "# Guide\n",
			expected:    false,
		},
		{
			description: "front matter requires content between delimiters",
			params:      []string{"front-matter:required"},
			content:     "---\n---\n",
			expected:    false,
		},
		{
			description: "unknown content subrule",
			params:      []string{"unknown:1"},
			expectedErr: "unknown content rule unknown",
		},
	}

	for _, test := range tests {
		contentRule := new(Content)
		contentRule.Init()

		err := contentRule.SetParameters(test.params)
		if test.expectedErr != "" {
			if err == nil || err.Error() != test.expectedErr {
				t.Fatalf("%s: expected error %q, got %v", test.description, test.expectedErr, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: expected no error, got %v", test.description, err)
		}

		if !reflect.DeepEqual(contentRule.GetParameters(), test.params) {
			t.Fatalf("%s: expected parameters %v, got %v", test.description, test.params, contentRule.GetParameters())
		}

		valid, err := contentRule.ValidateContent([]byte(test.content), "")
		if err != nil {
			t.Fatalf("%s: expected no validation error, got %v", test.description, err)
		}
		if valid != test.expected {
			t.Fatalf("%s: expected %t, got %t", test.description, test.expected, valid)
		}
	}
}

func TestNewPreparedContent(t *testing.T) {
	prepared := NewPreparedContent([]byte("\ufeff---\r\ntitle: Example\r\n---\r\n## Overview\r\nåäö\r\n"), PreparedContentOptions{
		MaxLineLength: true,
		FrontMatter:   true,
	})

	if prepared.lineCount != 5 {
		t.Fatalf("expected 5 lines, got %d", prepared.lineCount)
	}
	if prepared.maxLineLength != len("title: Example") {
		t.Fatalf("expected max line length %d, got %d", len("title: Example"), prepared.maxLineLength)
	}
	if !prepared.hasFrontMatter {
		t.Fatalf("expected prepared content to detect front matter")
	}
	expectedLines := []string{"---", "title: Example", "---", "## Overview", "åäö"}
	if !reflect.DeepEqual(prepared.lines, expectedLines) {
		t.Fatalf("expected lines %v, got %v", expectedLines, prepared.lines)
	}
}

func TestContent_GetPreparedContentOptions(t *testing.T) {
	tests := []struct {
		description string
		params      string
		expected    PreparedContentOptions
	}{
		{
			description: "max lines only needs line metadata",
			params:      "max-lines:10",
			expected:    PreparedContentOptions{},
		},
		{
			description: "max line length requests max length",
			params:      "max-line-length:10",
			expected:    PreparedContentOptions{MaxLineLength: true},
		},
		{
			description: "front matter requests front matter detection",
			params:      "front-matter:required",
			expected:    PreparedContentOptions{FrontMatter: true},
		},
	}

	for _, test := range tests {
		contentRule := new(Content)
		contentRule.Init()
		if err := contentRule.SetParameters([]string{test.params}); err != nil {
			t.Fatalf("%s: expected no error, got %v", test.description, err)
		}
		if !reflect.DeepEqual(contentRule.GetPreparedContentOptions(), test.expected) {
			t.Fatalf("%s: expected %+v, got %+v", test.description, test.expected, contentRule.GetPreparedContentOptions())
		}
	}
}

func TestContent_SetParametersErrors(t *testing.T) {
	tests := []struct {
		description string
		params      []string
		expectedErr string
	}{
		{
			description: "missing params",
			params:      nil,
			expectedErr: "content rule not exists",
		},
		{
			description: "empty max lines",
			params:      []string{"max-lines:"},
			expectedErr: "max-lines value is empty",
		},
		{
			description: "negative max lines",
			params:      []string{"max-lines:-1"},
			expectedErr: "max-lines value must be greater or equal to 0",
		},
		{
			description: "invalid heading regex",
			params:      []string{"heading:["},
			expectedErr: "error parsing regexp: missing closing ]: `[`",
		},
		{
			description: "front matter only supports required",
			params:      []string{"front-matter:optional"},
			expectedErr: "front-matter value must be required",
		},
	}

	for _, test := range tests {
		contentRule := new(Content)
		contentRule.Init()

		err := contentRule.SetParameters(test.params)
		if err == nil || err.Error() != test.expectedErr {
			t.Fatalf("%s: expected error %q, got %v", test.description, test.expectedErr, err)
		}
	}
}
