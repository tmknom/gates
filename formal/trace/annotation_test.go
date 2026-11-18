package main

import (
	"reflect"
	"testing"
)

func TestParseAnnotations(t *testing.T) {
	cases := []struct {
		name         string
		path         string
		content      string
		want         []annotation
		wantFindings []finding
	}{
		{
			name:    "SMT-LIB formalizes before define-fun is an annotation",
			path:    "m.smt2",
			content: lines("; trace:formalizes RFC7519-AUD-INTENDED RFC7519-AUD-REJECT", "(define-fun f () Bool true)"),
			want:    []annotation{{kind: formalizes, ids: []string{"RFC7519-AUD-INTENDED", "RFC7519-AUD-REJECT"}, path: "m.smt2", line: 1}},
		},
		{
			name:         "SMT-LIB formalizes not before define-fun is reported",
			path:         "m.smt2",
			content:      lines("; trace:formalizes GOJ-AUD", "; comment", "(define-fun f () Bool true)"),
			want:         []annotation{{kind: formalizes, ids: []string{"GOJ-AUD"}, path: "m.smt2", line: 1}},
			wantFindings: []finding{{path: "m.smt2", line: 1, message: "trace:formalizes must be on the line just before a definition"}},
		},
		{
			name:    "SMT-LIB checks on the line after expect is an annotation",
			path:    "c.smt2",
			content: lines("; expect: unsat", "; trace:checks GOJ-AUD RFC7519-AUD-REJECT", "(check-sat)"),
			want:    []annotation{{kind: checks, ids: []string{"GOJ-AUD", "RFC7519-AUD-REJECT"}, path: "c.smt2", line: 2, universal: true}},
		},
		{
			name:    "SMT-LIB checks expecting any sat is not universal",
			path:    "c.smt2",
			content: lines("; expect: unsat sat", "; trace:checks GOJ-AUD", "(check-sat)", "(check-sat)"),
			want:    []annotation{{kind: checks, ids: []string{"GOJ-AUD"}, path: "c.smt2", line: 2}},
		},
		{
			name:         "SMT-LIB checks below the second line is reported",
			path:         "c.smt2",
			content:      lines("; expect: unsat", "; comment", "; trace:checks GOJ-AUD"),
			want:         []annotation{{kind: checks, ids: []string{"GOJ-AUD"}, path: "c.smt2", line: 3}},
			wantFindings: []finding{{path: "c.smt2", line: 3, message: `trace:checks must be on the line after "; expect:"`}},
		},
		{
			name:         "SMT-LIB checks without expect on the first line is reported",
			path:         "c.smt2",
			content:      lines("; note", "; trace:checks GOJ-AUD"),
			want:         []annotation{{kind: checks, ids: []string{"GOJ-AUD"}, path: "c.smt2", line: 2}},
			wantFindings: []finding{{path: "c.smt2", line: 2, message: `trace:checks must be on the line after "; expect:"`}},
		},
		{
			name:    "SMT-LIB unformalized with a reason is an annotation of its ID",
			path:    "m.smt2",
			content: lines("; trace:unformalized RFC8725-UTF8-APPLICATION モデルはデコード後の値を扱う"),
			want:    []annotation{{kind: unformalized, ids: []string{"RFC8725-UTF8-APPLICATION"}, path: "m.smt2", line: 1}},
		},
		{
			name:         "unformalized without a reason is reported",
			path:         "m.smt2",
			content:      lines("; trace:unformalized RFC8725-UTF8-APPLICATION"),
			want:         []annotation{{kind: unformalized, ids: []string{"RFC8725-UTF8-APPLICATION"}, path: "m.smt2", line: 1}},
			wantFindings: []finding{{path: "m.smt2", line: 1, message: "trace:unformalized needs one ID and a reason"}},
		},
		{
			name:         "unformalized starting with a non-ID is reported",
			path:         "m.smt2",
			content:      lines("; trace:unformalized 理由だけ"),
			wantFindings: []finding{{path: "m.smt2", line: 1, message: "trace:unformalized needs one ID and a reason"}},
		},
		{
			name:         "unknown kind is reported",
			path:         "m.smt2",
			content:      lines("; trace:check GOJ-AUD"),
			wantFindings: []finding{{path: "m.smt2", line: 1, message: `unknown annotation "trace:check"`}},
		},
		{
			name:         "annotation without IDs is reported",
			path:         "m.smt2",
			content:      lines("; trace:formalizes", "(define-fun f () Bool true)"),
			wantFindings: []finding{{path: "m.smt2", line: 1, message: "trace:formalizes lists no ID"}},
		},
		{
			name:         "token that is not an ID is reported",
			path:         "m.smt2",
			content:      lines("; trace:formalizes GOJ_AUD", "(define-fun f () Bool true)"),
			want:         []annotation{{kind: formalizes, path: "m.smt2", line: 1}},
			wantFindings: []finding{{path: "m.smt2", line: 1, message: `"GOJ_AUD" is not an ID`}},
		},
		{
			name:    "Alloy formalizes before pred is an annotation",
			path:    "m.als",
			content: lines("// trace:formalizes GOJ-JWKS", "pred p[k: Key] { k in Key }"),
			want:    []annotation{{kind: formalizes, ids: []string{"GOJ-JWKS"}, path: "m.als", line: 1}},
		},
		{
			name:    "Alloy checks before a labeled check command is an annotation",
			path:    "m.als",
			content: lines("// trace:checks GOJ-JWKS", "NoForeignKey: check { all k: Key | p[k] } for 4"),
			want:    []annotation{{kind: checks, ids: []string{"GOJ-JWKS"}, path: "m.als", line: 1, universal: true}},
		},
		{
			name:         "Alloy checks not before a command is reported",
			path:         "m.als",
			content:      lines("// trace:checks GOJ-JWKS", "pred p { }"),
			want:         []annotation{{kind: checks, ids: []string{"GOJ-JWKS"}, path: "m.als", line: 1}},
			wantFindings: []finding{{path: "m.als", line: 1, message: "trace:checks must be on the line just before a check or run command"}},
		},
		{
			name:    "Go checks directive is an annotation",
			path:    "x_test.go",
			content: lines("//trace:checks AC-AUTHN-002 AC-AUTHZ-001", "func TestX(t *testing.T) {}"),
			want:    []annotation{{kind: checks, ids: []string{"AC-AUTHN-002", "AC-AUTHZ-001"}, path: "x_test.go", line: 1}},
		},
		{
			name:    "Go comment with a space is not an annotation",
			path:    "x_test.go",
			content: lines("// trace:checks is written without a space in Go"),
		},
		{
			name:    "Terraform test checks is an annotation",
			path:    "t.tftest.hcl",
			content: lines("# trace:checks AC-OPS-005", `run "x" {`),
			want:    []annotation{{kind: checks, ids: []string{"AC-OPS-005"}, path: "t.tftest.hcl", line: 1}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, findings := parseAnnotations(tc.path, tc.content)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("annotations = %+v, want %+v", got, tc.want)
			}
			if !reflect.DeepEqual(findings, tc.wantFindings) {
				t.Errorf("findings = %+v, want %+v", findings, tc.wantFindings)
			}
		})
	}
}
