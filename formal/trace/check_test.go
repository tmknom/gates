package main

import (
	"fmt"
	"reflect"
	"testing"
	"testing/fstest"
)

// fixtureDocuments are the documents of the ID kinds the fixture defines.
// The other kinds get a document that does not exist.
var fixtureDocuments = map[string]string{
	"AC":      "`acceptance-criteria.md`",
	"GOJ":     "`oidc-jwt-claims.md`、`oidc-jwt-signature.md`",
	"RFC7519": "`docs/external/standards/rfc7519.md`",
	"GH-OIDC": "`docs/external/github/oidc.md`",
}

// idKindsTable returns the table of ID kinds of the Traceability Policy for
// the prefixes given.
func idKindsTable(prefixes []string) string {
	table := []string{"## 1. ID Kinds", "", "| 接頭辞 | 対象 | 定めるドキュメント | ID の形 |", "|---|---|---|---|"}
	for _, p := range prefixes {
		documents, ok := fixtureDocuments[p]
		if !ok {
			documents = "`unused.md`"
		}
		table = append(table, fmt.Sprintf("| `%s-` | 対象 | %s | 名前 |", p, documents))
	}
	return lines(table...)
}

// The line numbers of the fixture that findings point to.
const (
	claimsSatisfiesLine  = 9
	claimsAcceptsLine    = 11
	claimsPossiblyLine   = 12
	claimsNotAppliedLine = 17
	jwtRejectLine        = 1
)

// repository returns a repository in which every link holds, with files
// overridden or added by files.
func repository(files map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{
		idKindsDocument: {Data: []byte(idKindsTable(idPrefixes))},
		actorsDocument: {Data: []byte(lines(
			"### "+actorSection,
			"- **Recipient**：受け取る主体",
			"",
			"## Claims",
			"### GOJ-AUD `aud`",
			"",
			"| Relation | Clause | Note |",
			"|---|---|---|",
			"| Satisfies | RFC7519-AUD-REJECT | なし |",
			"| Narrows | RFC7519-AUD-FORMAT | 理由 |",
			"| Accepts | GH-OIDC-AUD-CUSTOM | なし |",
			"| Possibly Rejects | GH-OIDC-AUD-X | 理由 |",
			"",
			"## "+notAppliedSection,
			"| Clause | Reason |",
			"|---|---|",
			"| RFC7519-AUD-NA | 理由 |",
		))},
		"docs/external/standards/rfc7519.md": {Data: []byte(lines(
			"### RFC7519-AUD-REJECT 拒否",
			"- **Level**：MUST",
			"- **Actor**：Recipient",
			"### RFC7519-AUD-FORMAT 形式",
			"- **Level**：Definition",
			"### RFC7519-AUD-NA 適用しない",
			"- **Level**：MUST",
			"- **Actor**：Recipient",
			"### RFC7519-ISS-ISSUER 発行者の義務",
			"- **Level**：MUST",
			"- **Actor**：Issuer",
		))},
		"docs/external/github/oidc.md": {Data: []byte(lines(
			"### GH-OIDC-AUD-CUSTOM 指定した Audience",
			"- **Evidence**：Documented",
			"### GH-OIDC-AUD-X 未確認の Audience",
			"- **Evidence**：Unconfirmed",
		))},
		"docs/spec/acceptance-criteria.md": {Data: []byte(lines("#### AC-AUTHN-001 例"))},
		"formal/z3/model/standards.smt2": {Data: []byte(lines(
			"; trace:formalizes RFC7519-AUD-REJECT",
			"(define-fun std-aud-reject () Bool true)",
			"; trace:formalizes RFC7519-AUD-FORMAT",
			"(define-fun std-aud-format () Bool true)",
			"; trace:unformalized RFC7519-AUD-NA 適用しない",
			"; trace:unformalized RFC7519-ISS-ISSUER 発行者の義務",
		))},
		"formal/z3/model/github.smt2": {Data: []byte(lines(
			"; trace:formalizes GH-OIDC-AUD-CUSTOM",
			"(define-fun gh-aud-custom () Bool true)",
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
		"formal/z3/checks/compat/aud-custom.smt2": {Data: []byte(lines(
			"; expect: unsat",
			"; trace:checks GOJ-AUD GH-OIDC-AUD-CUSTOM",
			"(check-sat)",
		))},
		"formal/z3/checks/compat/aud-x.smt2": {Data: []byte(lines(
			"; expect: sat",
			"; trace:checks GOJ-AUD GH-OIDC-AUD-X",
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

// goTest returns a Go test file whose test declares annotation.
func goTest(annotation string) string {
	return lines(annotation, "func TestZ(t *testing.T) {}")
}

// claimsWithRows returns the OIDC JWT Claims of the fixture with the
// relation rows of GOJ-AUD replaced by rows.
func claimsWithRows(rows ...string) string {
	content := []string{
		"### " + actorSection,
		"- **Recipient**：受け取る主体",
		"",
		"## Claims",
		"### GOJ-AUD `aud`",
		"",
		"| Relation | Clause | Note |",
		"|---|---|---|",
	}
	content = append(content, rows...)
	content = append(content, "", "## "+notAppliedSection, "| Clause | Reason |", "|---|---|", "| RFC7519-AUD-NA | 理由 |")
	return lines(content...)
}

const (
	satisfiesRow = "| Satisfies | RFC7519-AUD-REJECT | なし |"
	narrowsRow   = "| Narrows | RFC7519-AUD-FORMAT | 理由 |"
	acceptsRow   = "| Accepts | GH-OIDC-AUD-CUSTOM | なし |"
	possiblyRow  = "| Possibly Rejects | GH-OIDC-AUD-X | 理由 |"
)

func TestCheck(t *testing.T) {
	cases := []struct {
		name   string
		files  map[string]string
		remove []string
		want   []finding
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
			files: map[string]string{"internal/x/y_test.go": goTest("//trace:checks AC-AUTHN-999")},
			want:  []finding{{path: "internal/x/y_test.go", line: 1, message: "undefined ID AC-AUTHN-999"}},
		},
		{
			name:  "undefined ID in a Terraform test annotation is reported",
			files: map[string]string{"terraform/tests/kms.tftest.hcl": lines("# trace:checks AC-OPS-999", `run "x" {`)},
			want:  []finding{{path: "terraform/tests/kms.tftest.hcl", line: 1, message: "undefined ID AC-OPS-999"}},
		},
		{
			name:  "malformed annotation is reported",
			files: map[string]string{"formal/z3/model/x.smt2": lines("; trace:formalize GOJ-AUD")},
			want:  []finding{{path: "formal/z3/model/x.smt2", line: 1, message: `unknown annotation "trace:formalize"`}},
		},
		{
			name:  "ID defined twice is reported at the second definition",
			files: map[string]string{"docs/spec/acceptance-criteria.md": lines("#### AC-AUTHN-001 例", "#### AC-AUTHN-001 重複")},
			want:  []finding{{path: "docs/spec/acceptance-criteria.md", line: 2, message: "AC-AUTHN-001 is also defined at docs/spec/acceptance-criteria.md:1"}},
		},
		{
			name:  "ID defined outside the documents of its kind is reported",
			files: map[string]string{"docs/design/domain.md": lines("#### AC-AUTHN-002 置き場所の誤り")},
			want:  []finding{{path: "docs/design/domain.md", line: 1, message: "AC-AUTHN-002 is defined outside the documents for AC- IDs"}},
		},
		{
			name:  "prefix missing from the table of ID kinds is reported",
			files: map[string]string{idKindsDocument: idKindsTable(idPrefixes[1:])},
			want:  []finding{{path: idKindsDocument, line: 1, message: "the table of ID kinds has no row for FR-"}},
		},
		{
			name:  "prefix the check does not know in the table of ID kinds is reported",
			files: map[string]string{idKindsDocument: idKindsTable(append([]string{"XR"}, idPrefixes...))},
			want:  []finding{{path: idKindsDocument, line: 5, message: "XR- in the table of ID kinds is not an ID kind the trace check reads"}},
		},
		{
			name:   "missing table of ID kinds is reported",
			remove: []string{idKindsDocument},
			want:   []finding{{path: idKindsDocument, line: 1, message: "no table of ID kinds"}},
		},
		{
			name:  "GOJ without a relation table is reported",
			files: map[string]string{"docs/design/oidc-jwt-signature.md": lines("### GOJ-SIG 署名"), "formal/z3/model/sig.smt2": lines("; trace:formalizes GOJ-SIG", "(define-fun f () Bool true)")},
			want:  []finding{{path: "docs/design/oidc-jwt-signature.md", line: 1, message: "GOJ-SIG has no relation table"}},
		},
		{
			name:  "relation row with an empty Note is reported",
			files: map[string]string{actorsDocument: claimsWithRows("| Satisfies | RFC7519-AUD-REJECT |  |", narrowsRow, acceptsRow, possiblyRow)},
			want:  []finding{{path: actorsDocument, line: claimsSatisfiesLine, message: "relation row has an empty Note"}},
		},
		{
			name:  "None row with a clause is reported",
			files: map[string]string{actorsDocument: claimsWithRows(satisfiesRow, narrowsRow, acceptsRow, possiblyRow, "| None | RFC7519-AUD-FORMAT | なし |")},
			want: []finding{
				{path: actorsDocument, line: 13, message: `None row has Clause "RFC7519-AUD-FORMAT", want "なし"`},
				{path: actorsDocument, line: 13, message: "None row is not the only relation row of GOJ-AUD"},
			},
		},
		{
			name:  "Satisfies row with a GitHub clause is reported",
			files: map[string]string{actorsDocument: claimsWithRows(satisfiesRow, narrowsRow, acceptsRow, possiblyRow, "| Satisfies | GH-OIDC-AUD-CUSTOM | なし |")},
			want:  []finding{{path: actorsDocument, line: 13, message: "Satisfies row has GH-OIDC-AUD-CUSTOM, want a standard clause"}},
		},
		{
			name:  "Accepts row with a standard clause is reported",
			files: map[string]string{actorsDocument: claimsWithRows(satisfiesRow, narrowsRow, acceptsRow, possiblyRow, "| Accepts | RFC7519-AUD-REJECT | なし |")},
			want:  []finding{{path: actorsDocument, line: 13, message: "Accepts row has RFC7519-AUD-REJECT, want a GitHub clause"}},
		},
		{
			name:  "relation row whose Clause is not an ID is reported",
			files: map[string]string{actorsDocument: claimsWithRows(satisfiesRow, narrowsRow, acceptsRow, possiblyRow, "| Narrows | aud の形式 | 理由 |")},
			want:  []finding{{path: actorsDocument, line: 13, message: `Clause "aud の形式" is not an ID`}},
		},
		{
			name:  "Accepts and Possibly Rejects rows of the same clause are reported",
			files: map[string]string{actorsDocument: claimsWithRows(satisfiesRow, narrowsRow, acceptsRow, possiblyRow, "| Possibly Rejects | GH-OIDC-AUD-CUSTOM | 理由 |")},
			want: []finding{
				{path: actorsDocument, line: 13, message: "GOJ-AUD both Accepts and Possibly Rejects GH-OIDC-AUD-CUSTOM"},
				{path: actorsDocument, line: 13, message: "Possibly Rejects GOJ-AUD GH-OIDC-AUD-CUSTOM has no trace:checks listing both IDs on a check expecting an instance"},
			},
		},
		{
			name: "Satisfies row without a check listing both IDs is reported",
			files: map[string]string{"formal/z3/checks/safety/aud.smt2": lines(
				"; expect: unsat",
				"; trace:checks GOJ-AUD",
				"(check-sat)",
			)},
			want: []finding{{path: actorsDocument, line: claimsSatisfiesLine, message: "Satisfies GOJ-AUD RFC7519-AUD-REJECT has no trace:checks listing both IDs on a check expecting no counterexample"}},
		},
		{
			name: "Satisfies row checked only by a Go test is reported",
			files: map[string]string{
				"formal/z3/checks/safety/aud.smt2": lines("; expect: unsat", "(check-sat)"),
				"internal/x/z_test.go":             goTest("//trace:checks GOJ-AUD RFC7519-AUD-REJECT"),
			},
			want: []finding{{path: actorsDocument, line: claimsSatisfiesLine, message: "Satisfies GOJ-AUD RFC7519-AUD-REJECT has no trace:checks listing both IDs on a check expecting no counterexample"}},
		},
		{
			name: "Satisfies row checked only by a check expecting sat is reported",
			files: map[string]string{"formal/z3/checks/safety/aud.smt2": lines(
				"; expect: sat",
				"; trace:checks GOJ-AUD RFC7519-AUD-REJECT",
				"(check-sat)",
			)},
			want: []finding{{path: actorsDocument, line: claimsSatisfiesLine, message: "Satisfies GOJ-AUD RFC7519-AUD-REJECT has no trace:checks listing both IDs on a check expecting no counterexample"}},
		},
		{
			name: "Satisfies row checked by an Alloy check command holds",
			files: map[string]string{
				"formal/z3/checks/safety/aud.smt2": lines("; expect: unsat", "(check-sat)"),
				"formal/alloy/m.als":               lines("// trace:checks GOJ-AUD RFC7519-AUD-REJECT", "AudOK: check { } for 3 expect 0"),
			},
		},
		{
			name: "Satisfies row checked only by an Alloy run command is reported",
			files: map[string]string{
				"formal/z3/checks/safety/aud.smt2": lines("; expect: unsat", "(check-sat)"),
				"formal/alloy/m.als":               lines("// trace:checks GOJ-AUD RFC7519-AUD-REJECT", "AudWitness: run { } for 3 expect 1"),
			},
			want: []finding{{path: actorsDocument, line: claimsSatisfiesLine, message: "Satisfies GOJ-AUD RFC7519-AUD-REJECT has no trace:checks listing both IDs on a check expecting no counterexample"}},
		},
		{
			name: "Satisfies row with only a formalizes listing both IDs is reported",
			files: map[string]string{
				"formal/z3/checks/safety/aud.smt2": lines("; expect: unsat", "(check-sat)"),
				"formal/z3/model/both.smt2":        lines("; trace:formalizes GOJ-AUD RFC7519-AUD-REJECT", "(define-fun f () Bool true)"),
			},
			want: []finding{{path: actorsDocument, line: claimsSatisfiesLine, message: "Satisfies GOJ-AUD RFC7519-AUD-REJECT has no trace:checks listing both IDs on a check expecting no counterexample"}},
		},
		{
			name: "Satisfies row of an unformalized clause checked by a Go test holds",
			files: map[string]string{
				actorsDocument:                     claimsWithRows(satisfiesRow, narrowsRow, acceptsRow, possiblyRow, "| Satisfies | RFC7519-ISS-ISSUER | なし |"),
				"internal/x/z_test.go":             goTest("//trace:checks GOJ-AUD RFC7519-ISS-ISSUER"),
				"formal/z3/checks/safety/aud.smt2": lines("; expect: unsat", "; trace:checks GOJ-AUD RFC7519-AUD-REJECT", "(check-sat)"),
			},
		},
		{
			name: "Satisfies row of an unformalized clause declared only in Go code that is not a test is reported",
			files: map[string]string{
				actorsDocument:    claimsWithRows(satisfiesRow, narrowsRow, acceptsRow, possiblyRow, "| Satisfies | RFC7519-ISS-ISSUER | なし |"),
				"internal/x/z.go": lines("//trace:checks GOJ-AUD RFC7519-ISS-ISSUER", "func TestZ() {}"),
			},
			want: []finding{{path: actorsDocument, line: 13, message: "Satisfies GOJ-AUD RFC7519-ISS-ISSUER has no trace:checks listing both IDs on a Go test or a check expecting no counterexample"}},
		},
		{
			name: "Satisfies row checked only by an Alloy check command without expect is reported",
			files: map[string]string{
				"formal/z3/checks/safety/aud.smt2": lines("; expect: unsat", "(check-sat)"),
				"formal/alloy/m.als":               lines("// trace:checks GOJ-AUD RFC7519-AUD-REJECT", "AudOK: check { } for 3"),
			},
			want: []finding{{path: actorsDocument, line: claimsSatisfiesLine, message: "Satisfies GOJ-AUD RFC7519-AUD-REJECT has no trace:checks listing both IDs on a check expecting no counterexample"}},
		},
		{
			name:  "Satisfies row of an unformalized clause without a Go test is reported",
			files: map[string]string{actorsDocument: claimsWithRows(satisfiesRow, narrowsRow, acceptsRow, possiblyRow, "| Satisfies | RFC7519-ISS-ISSUER | なし |")},
			want:  []finding{{path: actorsDocument, line: 13, message: "Satisfies GOJ-AUD RFC7519-ISS-ISSUER has no trace:checks listing both IDs on a Go test or a check expecting no counterexample"}},
		},
		{
			name:  "Accepts row without a formal check listing both IDs is reported",
			files: map[string]string{"formal/z3/checks/compat/aud-custom.smt2": lines("; expect: unsat", "; trace:checks GOJ-AUD", "(check-sat)")},
			want:  []finding{{path: actorsDocument, line: claimsAcceptsLine, message: "Accepts GOJ-AUD GH-OIDC-AUD-CUSTOM has no trace:checks listing both IDs on a formal check"}},
		},
		{
			name:  "Accepts row checked only by a Go test is reported",
			files: map[string]string{"formal/z3/checks/compat/aud-custom.smt2": lines("; expect: unsat", "(check-sat)"), "internal/x/z_test.go": goTest("//trace:checks GOJ-AUD GH-OIDC-AUD-CUSTOM")},
			want:  []finding{{path: actorsDocument, line: claimsAcceptsLine, message: "Accepts GOJ-AUD GH-OIDC-AUD-CUSTOM has no trace:checks listing both IDs on a formal check"}},
		},
		{
			name: "Accepts row of an unformalized GOJ checked by a Go test holds",
			files: map[string]string{
				"formal/z3/checks/compat/aud-custom.smt2": lines("; expect: unsat", "(check-sat)"),
				"formal/z3/model/gates.smt2":              lines("; trace:unformalized GOJ-AUD 理由"),
				"internal/x/z_test.go":                    goTest("//trace:checks GOJ-AUD GH-OIDC-AUD-CUSTOM RFC7519-AUD-REJECT"),
			},
		},
		{
			name: "Accepts row of an unformalized GOJ without a Go test is reported",
			files: map[string]string{
				"formal/z3/checks/compat/aud-custom.smt2": lines("; expect: unsat", "(check-sat)"),
				"formal/z3/model/gates.smt2":              lines("; trace:unformalized GOJ-AUD 理由"),
				"internal/x/z_test.go":                    goTest("//trace:checks GOJ-AUD RFC7519-AUD-REJECT"),
			},
			want: []finding{{path: actorsDocument, line: claimsAcceptsLine, message: "Accepts GOJ-AUD GH-OIDC-AUD-CUSTOM has no trace:checks listing both IDs on a Go test or a formal check"}},
		},
		{
			name: "Accepts row checked by a check expecting both verdicts holds",
			files: map[string]string{"formal/z3/checks/compat/aud-custom.smt2": lines(
				"; expect: unsat sat",
				"; trace:checks GOJ-AUD GH-OIDC-AUD-CUSTOM",
				"(check-sat)",
				"(check-sat)",
			)},
		},
		{
			name:  "Possibly Rejects row without a check expecting an instance is reported",
			files: map[string]string{"formal/z3/checks/compat/aud-x.smt2": lines("; expect: unsat", "; trace:checks GOJ-AUD GH-OIDC-AUD-X", "(check-sat)")},
			want:  []finding{{path: actorsDocument, line: claimsPossiblyLine, message: "Possibly Rejects GOJ-AUD GH-OIDC-AUD-X has no trace:checks listing both IDs on a check expecting an instance"}},
		},
		{
			name:  "check listing a GOJ and a clause that no relation row links is reported",
			files: map[string]string{actorsDocument: claimsWithRows(satisfiesRow, narrowsRow, possiblyRow)},
			want:  []finding{{path: "formal/z3/checks/compat/aud-custom.smt2", line: 2, message: "trace:checks lists GOJ-AUD and GH-OIDC-AUD-CUSTOM, which no relation row of GOJ-AUD links"}},
		},
		{
			name: "check listing a clause that a Note of the GOJ names holds",
			files: map[string]string{
				actorsDocument:                         claimsWithRows(satisfiesRow, "| Narrows | RFC7519-AUD-FORMAT | RFC7519-ISS-ISSUER の上限に合わせる |", acceptsRow, possiblyRow),
				"formal/z3/checks/properties/aud.smt2": lines("; expect: unsat", "; trace:checks GOJ-AUD RFC7519-ISS-ISSUER", "(check-sat)"),
			},
		},
		{
			name:  "check expecting an instance for an Accepts relation is reported",
			files: map[string]string{"formal/z3/checks/compat/aud-custom-sat.smt2": lines("; expect: sat", "; trace:checks GOJ-AUD GH-OIDC-AUD-CUSTOM", "(check-sat)")},
			want:  []finding{{path: "formal/z3/checks/compat/aud-custom-sat.smt2", line: 2, message: "trace:checks expects an instance for GOJ-AUD and GH-OIDC-AUD-CUSTOM, which is not a Possibly Rejects relation"}},
		},
		{
			name:  "strong norm of a GATES role that no relation or table lists is reported",
			files: map[string]string{actorsDocument: claimsWithRows(narrowsRow, acceptsRow, possiblyRow), "formal/z3/checks/safety/aud.smt2": lines("; expect: unsat", "(check-sat)")},
			want:  []finding{{path: "docs/external/standards/rfc7519.md", line: jwtRejectLine, message: "RFC7519-AUD-REJECT is a strong norm of a GATES role, but no Satisfies or Narrows row and no table of standard clauses not applied lists it"}},
		},
		{
			name: "strong norm of an Actor that is not a GATES role needs no relation",
			files: map[string]string{actorsDocument: lines(
				"### "+actorSection,
				"- **Relying Party**：検証する主体",
				"### GOJ-AUD `aud`",
				"| Relation | Clause | Note |",
				"|---|---|---|",
				"| Accepts | GH-OIDC-AUD-CUSTOM | なし |",
				"| Possibly Rejects | GH-OIDC-AUD-X | 理由 |",
			), "formal/z3/checks/safety/aud.smt2": lines("; expect: unsat", "(check-sat)")},
		},
		{
			name: "clause both in a relation row and in the table of clauses not applied is reported",
			files: map[string]string{actorsDocument: claimsWithRows(satisfiesRow, narrowsRow, acceptsRow, possiblyRow) +
				lines("| RFC7519-AUD-REJECT | 理由 |")},
			want: []finding{{path: actorsDocument, line: claimsNotAppliedLine + 1, message: "RFC7519-AUD-REJECT is in the table of standard clauses not applied and in a Satisfies or Narrows row"}},
		},
		{
			name: "clause that is not a strong norm of a GATES role in the table of clauses not applied is reported",
			files: map[string]string{actorsDocument: claimsWithRows(satisfiesRow, narrowsRow, acceptsRow, possiblyRow) +
				lines("| RFC7519-ISS-ISSUER | 理由 |")},
			want: []finding{{path: actorsDocument, line: claimsNotAppliedLine + 1, message: "RFC7519-ISS-ISSUER in the table of standard clauses not applied is not a strong norm of a GATES role"}},
		},
		{
			name: "missing list of the Actors that are GATES roles is reported",
			files: map[string]string{actorsDocument: lines(
				"### GOJ-AUD `aud`",
				"| Relation | Clause | Note |",
				"|---|---|---|",
				satisfiesRow, narrowsRow, acceptsRow, possiblyRow,
			)},
			want: []finding{{path: actorsDocument, line: 1, message: "no list of the Actors that are GATES roles"}},
		},
		{
			name:  "clause neither formalized nor marked unformalized is reported",
			files: map[string]string{"formal/z3/model/github.smt2": lines("; no annotation")},
			want:  []finding{{path: "docs/external/github/oidc.md", line: 1, message: "GH-OIDC-AUD-CUSTOM has neither trace:formalizes nor trace:unformalized"}},
		},
		{
			name:  "GOJ neither formalized nor marked unformalized is reported",
			files: map[string]string{"formal/z3/model/gates.smt2": lines("; no annotation")},
			want:  []finding{{path: actorsDocument, line: 5, message: "GOJ-AUD has neither trace:formalizes nor trace:unformalized"}},
		},
		{
			name:  "GOJ formalized and marked unformalized is reported",
			files: map[string]string{"formal/z3/model/more.smt2": lines("; trace:unformalized GOJ-AUD 理由")},
			want:  []finding{{path: actorsDocument, line: 5, message: "GOJ-AUD has both trace:formalizes and trace:unformalized"}},
		},
		{
			name:  "Unconfirmed clause with an annotation is reported",
			files: map[string]string{"formal/z3/model/more.smt2": lines("; trace:unformalized GH-OIDC-AUD-X 理由")},
			want:  []finding{{path: "docs/external/github/oidc.md", line: 3, message: "GH-OIDC-AUD-X is Unconfirmed, so it takes neither trace:formalizes nor trace:unformalized"}},
		},
		{
			name:  "formalization declared only by a Go test is reported",
			files: map[string]string{"formal/z3/model/gates.smt2": lines("; no annotation"), "internal/x/z_test.go": lines("//trace:formalizes GOJ-AUD")},
			want:  []finding{{path: actorsDocument, line: 5, message: "GOJ-AUD has neither trace:formalizes nor trace:unformalized"}},
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
			fsys := repository(tc.files)
			for _, name := range tc.remove {
				delete(fsys, name)
			}
			got, err := check(fsys)
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
