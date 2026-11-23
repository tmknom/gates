package strictyaml

import (
	"slices"
	"testing"
)

func TestScanText(t *testing.T) {
	tests := []struct {
		name     string
		document string
		want     []string
	}{
		{name: "document without features has no condition", document: "repositories:\n  - id: \"1\"\n", want: nil},
		{name: "exclamation mark and percent sign in comments have no condition", document: "# ! %\nrepositories: [] # !!str %\n", want: nil},
		{name: "CRLF line breaks have no condition", document: "# !\r\nrepositories: []\r\n", want: nil},
		{name: "byte order mark at the start returns the condition", document: "\uFEFFrepositories: []\n", want: []string{"must not contain a byte order mark"}},
		{name: "byte order mark at the start of a later line returns the condition", document: "repositories:\n\uFEFF  - id: \"1\"\n", want: []string{"must not contain a byte order mark"}},
		{name: "NEL returns the condition", document: "# c\u0085repositories: []\n", want: []string{"must not contain NEL, LS, or PS"}},
		{name: "LS returns the condition", document: "# c\u2028repositories: []\n", want: []string{"must not contain NEL, LS, or PS"}},
		{name: "PS returns the condition", document: "# c\u2029repositories: []\n", want: []string{"must not contain NEL, LS, or PS"}},
		{name: "YAML directive returns the condition with its line", document: "%YAML 1.2\n---\nrepositories: []\n", want: []string{"line 1: must not have a directive"}},
		{name: "TAG directive returns the condition with its line", document: "# c\n%TAG !e! tag:example.com,2000:\n---\nrepositories: []\n", want: []string{"line 2: must not have a directive"}},
		{name: "secondary tag returns the condition with its line", document: "repositories:\n  - name: !!str a\n", want: []string{"line 2: must not have an explicit tag"}},
		{name: "non-specific tag returns the condition", document: "repositories:\n  - name: ! a\n", want: []string{"line 2: must not have an explicit tag"}},
		{name: "number sign without preceding space does not start a comment", document: "repositories: a#b !c\n", want: []string{"line 1: must not have an explicit tag"}},
		{name: "number sign after a tab starts a comment", document: "repositories: []\t# !c\n", want: nil},
		{name: "lines are counted across CRLF", document: "a: 1\r\nb: 2\r\nc: !x 3\r\n", want: []string{"line 3: must not have an explicit tag"}},
		{name: "lines are counted across CR", document: "a: 1\rb: !x 2\r", want: []string{"line 2: must not have an explicit tag"}},
		{name: "every condition is reported", document: "%YAML 1.2\n---\n\uFEFFa: !x 1\n", want: []string{"must not contain a byte order mark", "line 1: must not have a directive", "line 3: must not have an explicit tag"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Conditions
			if ok := scanText(tt.document, &c); !ok {
				t.Fatalf("scanText(%q) = false, want true", tt.document)
			}
			if !slices.Equal(c.List(), tt.want) {
				t.Errorf("scanText(%q) conditions = %q, want %q", tt.document, c.List(), tt.want)
			}
		})
	}
}

func TestScanText_InvalidUTF8(t *testing.T) {
	var c Conditions
	if ok := scanText("repositories: \xff !x\n", &c); ok {
		t.Error("scanText = true for invalid UTF-8, want false")
	}
	if want := []string{"must be valid UTF-8"}; !slices.Equal(c.List(), want) {
		t.Errorf("conditions = %q, want %q", c.List(), want)
	}
}
