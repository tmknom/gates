package strictyaml

import (
	"maps"
	"slices"
	"testing"
)

func TestReadFields(t *testing.T) {
	tests := []struct {
		name       string
		document   string
		path       string
		wantFields []string
		want       []string
	}{
		{name: "mapping with the keys is read", document: "v: {a: 1, b: 2}\n", path: "p", wantFields: []string{"a", "b"}},
		{name: "unknown key returns the condition without the key", document: "v: {a: 1, b: 2, secret_key: 3}\n", path: "p", wantFields: []string{"a", "b"}, want: []string{"p has an unknown key"}},
		{name: "unknown key of the document is labeled document", document: "v: {a: 1, b: 2, x: 3}\n", path: "", wantFields: []string{"a", "b"}, want: []string{"document has an unknown key"}},
		{name: "duplicate key differing only in quotes returns the condition", document: "v: {a: 1, \"a\": 2, b: 3}\n", path: "p", wantFields: []string{"a", "b"}, want: []string{"p.a is a duplicate key"}},
		{name: "missing key returns the condition", document: "v: {a: 1}\n", path: "p", wantFields: []string{"a"}, want: []string{"p.b is required"}},
		{name: "missing key of the document has no prefix", document: "v: {a: 1}\n", path: "", wantFields: []string{"a"}, want: []string{"b is required"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Conditions
			fields := ReadFields(value(t, tt.document), tt.path, []string{"a", "b"}, &c)
			if got := slices.Sorted(maps.Keys(fields)); !slices.Equal(got, tt.wantFields) {
				t.Errorf("fields = %q, want %q", got, tt.wantFields)
			}
			if !slices.Equal(c.List(), tt.want) {
				t.Errorf("conditions = %q, want %q", c.List(), tt.want)
			}
		})
	}
}

func TestReadFields_DuplicateKeepsTheFirstValue(t *testing.T) {
	var c Conditions
	fields := ReadFields(value(t, "v: {a: first, a: second}\n"), "p", []string{"a"}, &c)
	if got, _ := readString(fields["a"], "p.a", &c); got != "first" {
		t.Errorf("fields[a] = %q, want %q", got, "first")
	}
}

func TestReadFields_NotRead(t *testing.T) {
	tests := []struct {
		name string
		node func(t *testing.T) Node
		want []string
	}{
		{name: "sequence is not a mapping", node: func(t *testing.T) Node { return value(t, "v: [a]\n") }, want: []string{"p must be a mapping"}},
		{name: "missing value has no condition", node: func(*testing.T) Node { return Node{} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Conditions
			if fields := ReadFields(tt.node(t), "p", []string{"a"}, &c); fields != nil {
				t.Errorf("fields = %v, want nil", fields)
			}
			if !slices.Equal(c.List(), tt.want) {
				t.Errorf("conditions = %q, want %q", c.List(), tt.want)
			}
		})
	}
}

func TestReadMapping(t *testing.T) {
	tests := []struct {
		name         string
		document     string
		wantKeys     []string
		wantComplete bool
		want         []string
	}{
		{name: "string keys are read in order", document: "v: {b: 1, a: 2}\n", wantKeys: []string{"b", "a"}, wantComplete: true},
		{name: "quoted merge key is a string key", document: "v: {\"<<\": 1}\n", wantKeys: []string{"<<"}, wantComplete: true},
		{name: "unquoted merge key is rejected", document: "v: {<<: {}, a: 1}\n", wantKeys: []string{"a"}, want: []string{"line 1: must not have a merge key"}},
		{name: "integer key is not a string", document: "v: {1: a}\n", want: []string{"p has a key that is not a string"}},
		{name: "null key is not a string", document: "v: {~: a}\n", want: []string{"p has a key that is not a string"}},
		{name: "sequence key is not a string", document: "v: {[a]: b}\n", want: []string{"p has a key that is not a string"}},
		{name: "alias key is rejected", document: "v:\n  a: &k b\n  *k : c\n", wantKeys: []string{"a"}, want: []string{"line 3: must not be an alias"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Conditions
			pairs, complete, ok := readMapping(value(t, tt.document), "p", &c)
			if !ok {
				t.Fatal("readMapping ok = false, want true")
			}
			var keys []string
			for _, pair := range pairs {
				keys = append(keys, pair.key)
			}
			if !slices.Equal(keys, tt.wantKeys) || complete != tt.wantComplete {
				t.Errorf("readMapping = %q complete %v, want %q complete %v", keys, complete, tt.wantKeys, tt.wantComplete)
			}
			if !slices.Equal(c.List(), tt.want) {
				t.Errorf("conditions = %q, want %q", c.List(), tt.want)
			}
		})
	}
}

func TestReadSequence(t *testing.T) {
	tests := []struct {
		name     string
		document string
		wantLen  int
		want     []string
	}{
		{name: "items are returned", document: "v: [a, b]\n", wantLen: 2},
		{name: "empty sequence has no condition", document: "v: []\n", wantLen: 0},
		{name: "anchored sequence is read with the condition", document: "v: &s [a]\n", wantLen: 1, want: []string{"line 1: must not have an anchor"}},
		{name: "mapping is not a sequence", document: "v: {}\n", want: []string{"p must be a sequence"}},
		{name: "scalar is not a sequence", document: "v: a\n", want: []string{"p must be a sequence"}},
		{name: "null is not a sequence", document: "v: ~\n", want: []string{"p must be a sequence"}},
		{name: "empty value is not a sequence", document: "v:\n", want: []string{"p must be a sequence"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Conditions
			if got := ReadSequence(value(t, tt.document), "p", &c); len(got) != tt.wantLen {
				t.Errorf("len(ReadSequence) = %d, want %d", len(got), tt.wantLen)
			}
			if !slices.Equal(c.List(), tt.want) {
				t.Errorf("conditions = %q, want %q", c.List(), tt.want)
			}
		})
	}
}

func TestReadSequence_MissingValueHasNoCondition(t *testing.T) {
	var c Conditions
	if got := ReadSequence(Node{}, "p", &c); got != nil || len(c.List()) != 0 {
		t.Errorf("ReadSequence = %v with conditions %q, want nil without conditions", got, c.List())
	}
}

func TestReadString(t *testing.T) {
	tests := []struct {
		name     string
		document string
		want     string
	}{
		{name: "plain string", document: "v: read\n", want: "read"},
		{name: "YAML 1.1 boolean is a string", document: "v: yes\n", want: "yes"},
		{name: "form the YAML library reads as a date is a string", document: "v: 2001-12-14\n", want: "2001-12-14"},
		{name: "quoted digits are a string", document: "v: \"123\"\n", want: "123"},
		{name: "block scalar boolean is a string", document: "v: |-\n  true\n", want: "true"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Conditions
			got, ok := readString(value(t, tt.document), "p", &c)
			if !ok || got != tt.want || len(c.List()) != 0 {
				t.Errorf("readString = %q, %v with conditions %q, want %q", got, ok, c.List(), tt.want)
			}
		})
	}
}

func TestReadString_NotString(t *testing.T) {
	for _, document := range []string{"v: 123\n", "v: true\n", "v: ~\n", "v:\n", "v: [a]\n", "v: {}\n"} {
		t.Run(document, func(t *testing.T) {
			var c Conditions
			if _, ok := readString(value(t, document), "p", &c); ok {
				t.Error("readString ok = true, want false")
			}
			if want := []string{"p must be a string"}; !slices.Equal(c.List(), want) {
				t.Errorf("conditions = %q, want %q", c.List(), want)
			}
		})
	}
}

func TestReadQuoted(t *testing.T) {
	for _, document := range []string{"v: \"1\"\n", "v: '1'\n", "v: \"\\x31\"\n"} {
		t.Run(document, func(t *testing.T) {
			var c Conditions
			got, ok := readQuoted(value(t, document), "p", &c)
			if !ok || got != "1" || len(c.List()) != 0 {
				t.Errorf("readQuoted = %q, %v with conditions %q, want %q", got, ok, c.List(), "1")
			}
		})
	}
}

func TestReadQuoted_NotQuoted(t *testing.T) {
	for _, document := range []string{"v: 1\n", "v: |-\n  1\n", "v: [\"1\"]\n"} {
		t.Run(document, func(t *testing.T) {
			var c Conditions
			if _, ok := readQuoted(value(t, document), "p", &c); ok {
				t.Error("readQuoted ok = true, want false")
			}
			if want := []string{"p must be a quoted string"}; !slices.Equal(c.List(), want) {
				t.Errorf("conditions = %q, want %q", c.List(), want)
			}
		})
	}
}

func TestAccept(t *testing.T) {
	tests := []struct {
		name string
		node func(t *testing.T) Node
		ok   bool
		want []string
	}{
		{name: "value is accepted", node: func(t *testing.T) Node { return value(t, "v: a\n") }, ok: true},
		{name: "anchor is accepted with the condition", node: func(t *testing.T) Node { return value(t, "v: &x a\n") }, ok: true, want: []string{"line 1: must not have an anchor"}},
		{name: "alias is not read", node: func(t *testing.T) Node { return value(t, "a: &x b\nv: *x\n") }, want: []string{"line 2: must not be an alias"}},
		{name: "missing value is not read without a condition", node: func(*testing.T) Node { return Node{} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Conditions
			if ok := accept(tt.node(t), &c); ok != tt.ok {
				t.Errorf("accept = %v, want %v", ok, tt.ok)
			}
			if !slices.Equal(c.List(), tt.want) {
				t.Errorf("conditions = %q, want %q", c.List(), tt.want)
			}
		})
	}
}

func TestReadScalar_MissingValueHasNoCondition(t *testing.T) {
	var c Conditions
	if _, ok := readString(Node{}, "p", &c); ok {
		t.Error("readString ok = true, want false")
	}
	if _, ok := readQuoted(Node{}, "p", &c); ok {
		t.Error("readQuoted ok = true, want false")
	}
	if len(c.List()) != 0 {
		t.Errorf("conditions = %q, want none", c.List())
	}
}
