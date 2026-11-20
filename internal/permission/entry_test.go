package permission

import (
	"strings"
	"testing"
)

func TestParseEntry(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  entry
	}{
		{name: "permission name and level are parsed", input: "contents:read", want: entry{name: "contents", level: "read"}},
		{name: "permission name with underscore is parsed", input: "pull_requests:write", want: entry{name: "pull_requests", level: "write"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseEntry(tt.input)
			if err != nil {
				t.Fatalf("parseEntry(%q) returned error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("parseEntry(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseEntry_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
		// condition は、満たさなかった条件として文言に書かれる語である
		condition string
	}{
		{name: "entry without colon returns error", input: "contents", condition: "joined by ':'"},
		{name: "entry with two colons returns error", input: "contents:read:write", condition: "permission level must be"},
		{name: "uppercase permission name returns error", input: "Contents:read", condition: "permission name must be"},
		{name: "permission level with leading space returns error", input: "contents: read", condition: "permission level must be"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseEntry(tt.input)
			if err == nil {
				t.Fatalf("parseEntry(%q) returned no error", tt.input)
			}
			if !strings.Contains(err.Error(), tt.condition) {
				t.Errorf("parseEntry(%q) error = %q, want it to contain %q", tt.input, err, tt.condition)
			}
		})
	}
}

func TestParseEntry_ErrorDoesNotContainInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "permission name is not in error message", input: "secret-value:read"},
		{name: "permission level is not in error message", input: "contents:secret-value"},
		{name: "entry without colon is not in error message", input: "secret-value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseEntry(tt.input)
			if err == nil {
				t.Fatalf("parseEntry(%q) returned no error", tt.input)
			}
			if strings.Contains(err.Error(), "secret") {
				t.Errorf("error message %q contains the input", err.Error())
			}
		})
	}
}
