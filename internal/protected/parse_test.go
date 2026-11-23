package protected

import (
	"errors"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		document string
	}{
		{name: "document with comments is parsed", document: validDocument},
		{name: "empty repositories are parsed", document: "repositories: []\n"},
		{name: "document end marker is accepted", document: "---\nrepositories: []\n...\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.document)
			if err != nil {
				t.Fatalf("Parse returned error: %v", err)
			}
			if got.IsZero() {
				t.Error("IsZero() = true for a parsed list, want false")
			}
		})
	}
}

//trace:checks AC-AUTHZ-013
func TestParse_Invalid(t *testing.T) {
	tests := []struct {
		name     string
		document string
		want     string
	}{
		{name: "invalid UTF-8 stops before decoding", document: "\uFEFFrepositories: \xff\n", want: "must be valid UTF-8"},
		{name: "text and structure conditions are reported together", document: "\uFEFFrepositories: []\nversion: \"1\"\n", want: "must not contain a byte order mark; document has an unknown key"},
		{name: "text condition is reported with a syntax error", document: "a: !x [\n", want: "line 1: must not have an explicit tag; line 1: is not valid YAML"},
		{name: "document count and structure conditions are reported together", document: "repositories: {}\n---\n", want: "must be a single document; repositories must be a sequence"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.document)
			want := "invalid protected repositories: " + tt.want
			if !errors.Is(err, ErrInvalidRepositories) || err.Error() != want {
				t.Errorf("Parse error = %v, want %q", err, want)
			}
			if !got.IsZero() {
				t.Error("IsZero() = false for a rejected document, want true")
			}
		})
	}
}

//trace:checks AC-AUTHZ-013
func TestParse_ErrorDoesNotContainInput(t *testing.T) {
	tests := []struct {
		name     string
		document string
	}{
		{name: "syntax error", document: "repositories: [secret_value\n"},
		{name: "unknown keys", document: "repositories: []\nsecret_key: secret_value\n"},
		{name: "malformed ids", document: "repositories:\n  - id: \"secret_value\"\n    name: a\n    callers: []\n  - id: \"92233720368547758079\"\n    name: b\n    callers: []\n"},
		{name: "malformed and duplicate names", document: "repositories:\n  - id: \"1\"\n    name: secret/value\n    callers: []\n  - id: \"2\"\n    name: secret_value\n    callers: []\n  - id: \"3\"\n    name: SECRET_VALUE\n    callers: []\n"},
		{name: "permissions outside allowed permissions", document: replaceOnce(t, validDocument, "pull_requests: read", "secret_value: secret_level")},
		{name: "comments and keys in a rejected document", document: "# secret_value\nrepositories: {secret_key: secret_value}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.document)
			if err == nil {
				t.Fatal("Parse returned no error")
			}
			if msg := err.Error(); strings.Contains(strings.ToLower(msg), "secret") || strings.Contains(msg, "9223372036854775807") || strings.Contains(msg, "yaml:") {
				t.Errorf("error message %q contains the input or the library message", msg)
			}
		})
	}
}
