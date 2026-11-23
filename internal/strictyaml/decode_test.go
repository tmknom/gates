package strictyaml

import (
	"slices"
	"testing"
)

func TestDecodeDocument(t *testing.T) {
	var c Conditions
	if _, ok := decodeDocument("repositories: []\n", &c); !ok || len(c.List()) != 0 {
		t.Errorf("decodeDocument = %v with conditions %q, want true without conditions", ok, c.List())
	}
}

func TestDecodeDocument_Invalid(t *testing.T) {
	tests := []struct {
		name     string
		document string
		ok       bool
		want     []string
	}{
		{name: "empty string is not a mapping", document: "", want: []string{"document must be a mapping"}},
		{name: "comment only is not a mapping", document: "# repositories: []\n", want: []string{"document must be a mapping"}},
		{name: "syntax error returns its line without the library message", document: "repositories: []\nsecret: [\n", want: []string{"line 2: is not valid YAML"}},
		{name: "syntax error without a line returns no position", document: "repositories: \x00\n", want: []string{"is not valid YAML"}},
		{name: "second document returns the condition", document: "repositories: []\n---\nrepositories: []\n", ok: true, want: []string{"must be a single document"}},
		{name: "empty second document returns the condition", document: "repositories: []\n---\n", ok: true, want: []string{"must be a single document"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Conditions
			_, ok := decodeDocument(tt.document, &c)
			if ok != tt.ok {
				t.Errorf("decodeDocument(%q) = %v, want %v", tt.document, ok, tt.ok)
			}
			if !slices.Equal(c.List(), tt.want) {
				t.Errorf("decodeDocument(%q) conditions = %q, want %q", tt.document, c.List(), tt.want)
			}
		})
	}
}

func TestDecode(t *testing.T) {
	var c Conditions
	root, ok := Decode("v: a\n", &c)
	if !ok || len(c.List()) != 0 {
		t.Fatalf("Decode = %v with conditions %q, want true without conditions", ok, c.List())
	}
	if fields := ReadFields(root, "", []string{"v"}, &c); len(fields) != 1 || len(c.List()) != 0 {
		t.Errorf("ReadFields of the decoded root = %d fields with conditions %q, want the top-level mapping", len(fields), c.List())
	}
}

func TestDecode_Invalid(t *testing.T) {
	tests := []struct {
		name     string
		document string
		ok       bool
		want     []string
	}{
		{name: "invalid UTF-8 stops before decoding", document: "v: \xff [\n", want: []string{"must be valid UTF-8"}},
		{name: "unreadable document stops", document: "v: [\n", want: []string{"line 1: is not valid YAML"}},
		{name: "text and document count conditions are both kept", document: "v: !x a\n---\n", ok: true, want: []string{"line 1: must not have an explicit tag", "must be a single document"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Conditions
			if _, ok := Decode(tt.document, &c); ok != tt.ok {
				t.Errorf("Decode(%q) = %v, want %v", tt.document, ok, tt.ok)
			}
			if !slices.Equal(c.List(), tt.want) {
				t.Errorf("Decode(%q) conditions = %q, want %q", tt.document, c.List(), tt.want)
			}
		})
	}
}
