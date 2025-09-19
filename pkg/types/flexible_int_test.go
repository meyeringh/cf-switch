package types_test

import (
	"encoding/json"
	"testing"

	"github.com/meyeringh/cf-switch/pkg/types"
)

func TestFlexibleInt_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
		wantErr  bool
	}{
		{
			name:     "integer value",
			input:    `123`,
			expected: 123,
			wantErr:  false,
		},
		{
			name:     "string value",
			input:    `"456"`,
			expected: 456,
			wantErr:  false,
		},
		{
			name:     "zero integer",
			input:    `0`,
			expected: 0,
			wantErr:  false,
		},
		{
			name:     "zero string",
			input:    `"0"`,
			expected: 0,
			wantErr:  false,
		},
		{
			name:    "invalid string",
			input:   `"abc"`,
			wantErr: true,
		},
		{
			name:    "non-numeric non-string",
			input:   `true`,
			wantErr: true,
		},
		{
			name:     "null value",
			input:    `null`,
			expected: 0,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fi types.FlexibleInt
			err := json.Unmarshal([]byte(tt.input), &fi)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if fi.Int() != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, fi.Int())
			}
		})
	}
}

func TestFlexibleInt_MarshalJSON(t *testing.T) {
	fi := types.FlexibleInt(42)

	data, err := json.Marshal(fi)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	expected := `42`
	if string(data) != expected {
		t.Errorf("expected %s, got %s", expected, string(data))
	}
}

func TestCloudflareRule_WithStringVersion(t *testing.T) {
	// Test that CloudflareRule can unmarshal JSON with string version
	jsonData := `{
		"id": "test-rule",
		"action": "block",
		"expression": "http.host in {\"test.com\"}",
		"description": "test rule",
		"enabled": true,
		"version": "5"
	}`

	var rule types.CloudflareRule
	err := json.Unmarshal([]byte(jsonData), &rule)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if rule.Version.Int() != 5 {
		t.Errorf("expected version 5, got %d", rule.Version.Int())
	}
}

func TestCloudflareRule_WithIntVersion(t *testing.T) {
	// Test that CloudflareRule can unmarshal JSON with integer version
	jsonData := `{
		"id": "test-rule",
		"action": "block",
		"expression": "http.host in {\"test.com\"}",
		"description": "test rule",
		"enabled": true,
		"version": 7
	}`

	var rule types.CloudflareRule
	err := json.Unmarshal([]byte(jsonData), &rule)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if rule.Version.Int() != 7 {
		t.Errorf("expected version 7, got %d", rule.Version.Int())
	}
}
