package main

import (
	"reflect"
	"testing"
	"testing/fstest"
)

// repository returns a repository in which every link holds, with files
// overridden or added by files.
func repository(files map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{
		"docs/design/claims.md": {Data: []byte(lines(
			"### GOJ-AUD `aud`",
			"",
			"| Relation | Clause | Note |",
			"|---|---|---|",
			"| Satisfies | RFC7519-AUD-REJECT | なし |",
			"| Narrows | RFC7519-AUD-FORMAT | 理由 |",
		))},
		"docs/external/jwt.md": {Data: []byte(lines(
			"#### RFC7519-AUD-REJECT 拒否",
			"#### RFC7519-AUD-FORMAT 形式",
			"#### AC-AUTHN-001 例",
		))},
		"formal/z3/model/gates.smt2": {Data: []byte(lines(
			"; trace:formalizes GOJ-AUD",
			"(define-fun gates-aud () Bool true)",
		))},
		"formal/z3/checks/safety/aud.smt2": {Data: []byte(lines(
			"; expect: unsat",
			"; trace:checks GOJ-AUD RFC7519-AUD-REJECT",
			"(check-sat)",
		))},
		"internal/x/x_test.go":              {Data: []byte(lines("//trace:checks AC-AUTHN-001", "func TestX(t *testing.T) {}"))},
		".github/actions/a/a_test.go":       {Data: []byte(lines("package a"))},
		"terraform/tests/kms.tftest.hcl":    {Data: []byte(lines("# trace:checks AC-AUTHN-001", `run "x" {`))},
		"formal/z3/checks/safety/notes.txt": {Data: []byte(lines("GOJ-UNDEFINED in a file that is neither a document nor a model"))},
	}
	for name, content := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(content)}
	}
	return fsys
}

func TestCheck(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  []finding
	}{
		{
			name: "repository whose links all hold has no findings",
		},
		{
			name:  "undefined ID in a document is reported",
			files: map[string]string{"docs/spec/x.md": lines("GOJ-SUB を参照する")},
			want:  []finding{{path: "docs/spec/x.md", line: 1, message: "undefined ID GOJ-SUB"}},
		},
		{
			name:  "undefined ID in a formal document is reported",
			files: map[string]string{"formal/README.md": lines("GOJ-SUB を参照する")},
			want:  []finding{{path: "formal/README.md", line: 1, message: "undefined ID GOJ-SUB"}},
		},
		{
			name:  "undefined ID in a model comment is reported",
			files: map[string]string{"formal/alloy/m.als": lines("// see GOJ-SUB")},
			want:  []finding{{path: "formal/alloy/m.als", line: 1, message: "undefined ID GOJ-SUB"}},
		},
		{
			name:  "undefined ID in a model annotation is reported once",
			files: map[string]string{"formal/z3/model/x.smt2": lines("; trace:formalizes GOJ-SUB", "(define-fun f () Bool true)")},
			want:  []finding{{path: "formal/z3/model/x.smt2", line: 1, message: "undefined ID GOJ-SUB"}},
		},
		{
			name:  "undefined ID in a Go test annotation is reported",
			files: map[string]string{"internal/x/y_test.go": lines("//trace:checks AC-AUTHN-999")},
			want:  []finding{{path: "internal/x/y_test.go", line: 1, message: "undefined ID AC-AUTHN-999"}},
		},
		{
			name:  "undefined ID in a Terraform test annotation is reported",
			files: map[string]string{"terraform/tests/kms.tftest.hcl": lines("# trace:checks AC-OPS-999")},
			want:  []finding{{path: "terraform/tests/kms.tftest.hcl", line: 1, message: "undefined ID AC-OPS-999"}},
		},
		{
			name:  "malformed annotation is reported",
			files: map[string]string{"formal/z3/model/x.smt2": lines("; trace:formalize GOJ-AUD")},
			want:  []finding{{path: "formal/z3/model/x.smt2", line: 1, message: `unknown annotation "trace:formalize"`}},
		},
		{
			name: "Satisfies row without a check listing both IDs is reported",
			files: map[string]string{"formal/z3/checks/safety/aud.smt2": lines(
				"; expect: unsat",
				"; trace:checks GOJ-AUD",
				"(check-sat)",
			)},
			want: []finding{{path: "docs/design/claims.md", line: 5, message: "Satisfies GOJ-AUD RFC7519-AUD-REJECT has no trace:checks listing both IDs on a check expecting no counterexample"}},
		},
		{
			name: "Satisfies row checked only by a Go test is reported",
			files: map[string]string{
				"formal/z3/checks/safety/aud.smt2": lines("; expect: unsat", "(check-sat)"),
				"internal/x/z_test.go":             lines("//trace:checks GOJ-AUD RFC7519-AUD-REJECT"),
			},
			want: []finding{{path: "docs/design/claims.md", line: 5, message: "Satisfies GOJ-AUD RFC7519-AUD-REJECT has no trace:checks listing both IDs on a check expecting no counterexample"}},
		},
		{
			name: "Satisfies row checked only by a check expecting sat is reported",
			files: map[string]string{"formal/z3/checks/safety/aud.smt2": lines(
				"; expect: sat",
				"; trace:checks GOJ-AUD RFC7519-AUD-REJECT",
				"(check-sat)",
			)},
			want: []finding{{path: "docs/design/claims.md", line: 5, message: "Satisfies GOJ-AUD RFC7519-AUD-REJECT has no trace:checks listing both IDs on a check expecting no counterexample"}},
		},
		{
			name: "Satisfies row checked by an Alloy check command holds",
			files: map[string]string{
				"formal/z3/checks/safety/aud.smt2": lines("; expect: unsat", "(check-sat)"),
				"formal/alloy/m.als":               lines("// trace:checks GOJ-AUD RFC7519-AUD-REJECT", "AudOK: check { } for 3"),
			},
		},
		{
			name: "Satisfies row checked only by an Alloy run command is reported",
			files: map[string]string{
				"formal/z3/checks/safety/aud.smt2": lines("; expect: unsat", "(check-sat)"),
				"formal/alloy/m.als":               lines("// trace:checks GOJ-AUD RFC7519-AUD-REJECT", "AudWitness: run { } for 3"),
			},
			want: []finding{{path: "docs/design/claims.md", line: 5, message: "Satisfies GOJ-AUD RFC7519-AUD-REJECT has no trace:checks listing both IDs on a check expecting no counterexample"}},
		},
		{
			name: "Satisfies row checked without the GOJ ID is reported",
			files: map[string]string{"formal/z3/checks/safety/aud.smt2": lines(
				"; expect: unsat",
				"; trace:checks RFC7519-AUD-REJECT",
				"(check-sat)",
			)},
			want: []finding{{path: "docs/design/claims.md", line: 5, message: "Satisfies GOJ-AUD RFC7519-AUD-REJECT has no trace:checks listing both IDs on a check expecting no counterexample"}},
		},
		{
			name: "Satisfies row with only a formalizes listing both IDs is reported",
			files: map[string]string{
				"formal/z3/checks/safety/aud.smt2": lines("; expect: unsat", "(check-sat)"),
				"formal/z3/model/both.smt2":        lines("; trace:formalizes GOJ-AUD RFC7519-AUD-REJECT", "(define-fun f () Bool true)"),
			},
			want: []finding{{path: "docs/design/claims.md", line: 5, message: "Satisfies GOJ-AUD RFC7519-AUD-REJECT has no trace:checks listing both IDs on a check expecting no counterexample"}},
		},
		{
			name: "findings are sorted by path and line",
			files: map[string]string{
				"docs/spec/b.md": lines("", "GOJ-B"),
				"docs/spec/a.md": lines("GOJ-A2", "GOJ-A1"),
			},
			want: []finding{
				{path: "docs/spec/a.md", line: 1, message: "undefined ID GOJ-A2"},
				{path: "docs/spec/a.md", line: 2, message: "undefined ID GOJ-A1"},
				{path: "docs/spec/b.md", line: 2, message: "undefined ID GOJ-B"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := check(repository(tc.files))
			if err != nil {
				t.Fatalf("check() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("check() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestCheckMissingRoot(t *testing.T) {
	fsys := repository(nil)
	delete(fsys, "terraform/tests/kms.tftest.hcl")
	if _, err := check(fsys); err == nil {
		t.Error("check() error = nil, want an error for the missing terraform/tests")
	}
}
