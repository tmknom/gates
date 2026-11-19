package main

import (
	"reflect"
	"strings"
	"testing"
)

func lines(ls ...string) string {
	return strings.Join(ls, "\n") + "\n"
}

func TestParseMarkdownDefinitions(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    []definition
	}{
		{
			name:    "heading starting with an ID defines it",
			content: lines("# Doc", "", "### GOJ-AUD `aud`"),
			want:    []definition{{id: "GOJ-AUD", path: "d.md", line: 3}},
		},
		{
			name:    "heading made of the ID alone defines it",
			content: lines("## GAJ-ISS"),
			want:    []definition{{id: "GAJ-ISS", path: "d.md", line: 1}},
		},
		{
			name:    "heading in a fenced code block defines nothing",
			content: lines("```markdown", "#### RFC7519-AUD-REJECT 例", "```"),
			want:    nil,
		},
		{
			name:    "heading mentioning an ID after its title defines nothing",
			content: lines("### Rules for GOJ-AUD"),
			want:    nil,
		},
		{
			name: "items just below the heading are fields of the definition",
			content: lines(
				"#### RFC7519-AUD-REJECT 拒否",
				"",
				"- **Source**：[RFC 7519 §4.1.3](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.3)",
				"- **Level**：MUST",
				"- **Actor**：Recipient",
			),
			want: []definition{{
				id: "RFC7519-AUD-REJECT", path: "d.md", line: 1,
				fields: map[string]string{
					"Source": "[RFC 7519 §4.1.3](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.3)",
					"Level":  "MUST",
					"Actor":  "Recipient",
				},
			}},
		},
		{
			name: "items after the first paragraph are not fields",
			content: lines(
				"#### GH-OIDC-AUD-X 例",
				"",
				"- **Evidence**：Unconfirmed",
				"",
				"本文",
				"",
				"- **Evidence**：Documented",
			),
			want: []definition{{
				id: "GH-OIDC-AUD-X", path: "d.md", line: 1,
				fields: map[string]string{"Evidence": "Unconfirmed"},
			}},
		},
		{
			name: "items of a list after a blank line are not fields",
			content: lines(
				"#### GH-OIDC-AUD-X 例",
				"- **Evidence**：Unconfirmed",
				"",
				"- **Evidence**：Documented",
			),
			want: []definition{{
				id: "GH-OIDC-AUD-X", path: "d.md", line: 1,
				fields: map[string]string{"Evidence": "Unconfirmed"},
			}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseMarkdown("d.md", tc.content)
			if !reflect.DeepEqual(got.definitions, tc.want) {
				t.Errorf("definitions = %+v, want %+v", got.definitions, tc.want)
			}
		})
	}
}

func TestParseMarkdownReferences(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    []occurrence
	}{
		{
			name:    "ID in prose is a reference",
			content: lines("本文は GOJ-TIME を参照する"),
			want:    []occurrence{{id: "GOJ-TIME", path: "d.md", line: 1}},
		},
		{
			name:    "ID in a fenced code block is not a reference",
			content: lines("```", "| Accepts | GH-OIDC-AUD-CUSTOM | なし |", "```", "GOJ-AUD"),
			want:    []occurrence{{id: "GOJ-AUD", path: "d.md", line: 4}},
		},
		{
			name:    "ID in a tilde fence is not a reference",
			content: lines("~~~", "GH-OIDC-AUD-CUSTOM", "~~~"),
			want:    nil,
		},
		{
			name:    "tilde line inside a backtick fence does not close it",
			content: lines("```", "~~~", "GOJ-IN", "```", "GOJ-OUT"),
			want:    []occurrence{{id: "GOJ-OUT", path: "d.md", line: 5}},
		},
		{
			name:    "shorter backtick line inside a longer fence does not close it",
			content: lines("````markdown", "```", "GOJ-IN", "```", "````", "GOJ-OUT"),
			want:    []occurrence{{id: "GOJ-OUT", path: "d.md", line: 6}},
		},
		{
			name:    "backtick line with an info string does not close a fence",
			content: lines("```", "```smt2", "GOJ-IN", "```", "GOJ-OUT"),
			want:    []occurrence{{id: "GOJ-OUT", path: "d.md", line: 5}},
		},
		{
			name:    "indented fence in a list item is a code block",
			content: lines("- 例", "    ```", "    GOJ-IN", "    ```", "GOJ-OUT"),
			want:    []occurrence{{id: "GOJ-OUT", path: "d.md", line: 5}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseMarkdown("d.md", tc.content)
			if !reflect.DeepEqual(got.references, tc.want) {
				t.Errorf("references = %+v, want %+v", got.references, tc.want)
			}
		})
	}
}

func TestParseMarkdownRelations(t *testing.T) {
	cases := []struct {
		name         string
		content      string
		wantRelation []relation
		wantFindings []finding
	}{
		{
			name: "rows of the table under a definition are relations of it",
			content: lines(
				"### GOJ-AUD `aud`",
				"",
				"| Relation | Clause | Note |",
				"|---|---|---|",
				"| Satisfies | RFC7519-AUD-REJECT | なし |",
				"| Possibly Rejects | GH-OIDC-AUD-X | 理由 |",
				"",
				"本文",
			),
			wantRelation: []relation{
				{definition: "GOJ-AUD", kind: "Satisfies", clause: "RFC7519-AUD-REJECT", note: "なし", path: "d.md", line: 5},
				{definition: "GOJ-AUD", kind: "Possibly Rejects", clause: "GH-OIDC-AUD-X", note: "理由", path: "d.md", line: 6},
			},
		},
		{
			name: "table after the next heading belongs to that heading",
			content: lines(
				"### GOJ-EXP `exp`",
				"### GOJ-IAT `iat`",
				"| Relation | Clause | Note |",
				"|---|---|---|",
				"| None | なし | なし |",
			),
			wantRelation: []relation{
				{definition: "GOJ-IAT", kind: "None", clause: "なし", note: "なし", path: "d.md", line: 5},
			},
		},
		{
			name: "row with an empty Note is a relation with an empty note",
			content: lines(
				"### GOJ-AUD `aud`",
				"| Relation | Clause | Note |",
				"|---|---|---|",
				"| Satisfies | RFC7519-AUD-REJECT |  |",
			),
			wantRelation: []relation{
				{definition: "GOJ-AUD", kind: "Satisfies", clause: "RFC7519-AUD-REJECT", path: "d.md", line: 4},
			},
		},
		{
			name: "unknown Relation value is reported",
			content: lines(
				"### GOJ-AUD `aud`",
				"| Relation | Clause | Note |",
				"|---|---|---|",
				"| Satisfy | RFC7519-AUD-REJECT | なし |",
			),
			wantFindings: []finding{{path: "d.md", line: 4, message: `unknown Relation "Satisfy"`}},
		},
		{
			name: "row without three cells is reported",
			content: lines(
				"### GOJ-AUD `aud`",
				"| Relation | Clause | Note |",
				"|---|---|---|",
				"| Satisfies | RFC7519-AUD-REJECT |",
			),
			wantFindings: []finding{{path: "d.md", line: 4, message: "relation row has 2 cells, want Relation, Clause, and Note"}},
		},
		{
			name: "relation table outside a definition is reported",
			content: lines(
				"## Rules",
				"| Relation | Clause | Note |",
				"|---|---|---|",
				"| Satisfies | RFC7519-AUD-REJECT | なし |",
			),
			wantFindings: []finding{{path: "d.md", line: 2, message: "relation table is not under a definition heading"}},
		},
		{
			name: "table header that misspells the relation table is reported",
			content: lines(
				"### GOJ-AUD `aud`",
				"| Relation | Clauses | Note |",
				"|---|---|---|",
				"| Satisfies | RFC7519-AUD-REJECT | なし |",
			),
			wantFindings: []finding{{path: "d.md", line: 2, message: `table header "Relation | Clauses | Note" is not "Relation | Clause | Note"`}},
		},
		{
			name: "table header with a plural Relation column is reported",
			content: lines(
				"### GOJ-AUD `aud`",
				"| Relations | Clause | Note |",
				"|---|---|---|",
			),
			wantFindings: []finding{{path: "d.md", line: 2, message: `table header "Relations | Clause | Note" is not "Relation | Clause | Note"`}},
		},
		{
			name: "table header with a Relation-like column outside a definition is not reported",
			content: lines(
				"## Rules",
				"| Relations | Clause | Note |",
				"|---|---|---|",
			),
		},
		{
			name: "table without a Relation column under a GOJ is not reported",
			content: lines(
				"### GOJ-AUD `aud`",
				"| Claim | Type |",
				"|---|---|",
				"| aud | string |",
			),
		},
		{
			name: "Relationship table under a definition other than a GOJ is not reported",
			content: lines(
				"### AC-AUTHN-001 例",
				"| Relationship | Target | Target Type |",
				"|---|---|---|",
				"| Verifies | FR-AUTHN-001 | Functional Requirement |",
			),
		},
		{
			name: "table after a non-table line under the same definition is not a relation table",
			content: lines(
				"### GOJ-AUD `aud`",
				"| Relation | Clause | Note |",
				"|---|---|---|",
				"| Satisfies | RFC7519-AUD-REJECT | なし |",
				"",
				"| Claim | Type |",
				"| exp | number |",
			),
			wantRelation: []relation{
				{definition: "GOJ-AUD", kind: "Satisfies", clause: "RFC7519-AUD-REJECT", note: "なし", path: "d.md", line: 4},
			},
		},
		{
			name:    "relation table in a fenced code block is an example",
			content: lines("### GOJ-AUD `aud`", "```markdown", "| Relation | Clause | Note |", "|---|---|---|", "| Satisfies | RFC7519-AUD-REJECT | なし |", "```"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseMarkdown("d.md", tc.content)
			if !reflect.DeepEqual(got.relations, tc.wantRelation) {
				t.Errorf("relations = %+v, want %+v", got.relations, tc.wantRelation)
			}
			if !reflect.DeepEqual(got.findings, tc.wantFindings) {
				t.Errorf("findings = %+v, want %+v", got.findings, tc.wantFindings)
			}
		})
	}
}

func TestParseMarkdownNotApplied(t *testing.T) {
	cases := []struct {
		name         string
		content      string
		want         []occurrence
		wantFindings []finding
	}{
		{
			name: "rows of the table under Standard Clauses Not Applied are clauses not applied",
			content: lines(
				"## Standard Clauses Not Applied",
				"",
				"| Clause | Reason |",
				"|---|---|",
				"| OIDCC-NONCE-VALIDATE | 認証要求を前提とする |",
			),
			want: []occurrence{{id: "OIDCC-NONCE-VALIDATE", path: "d.md", line: 5}},
		},
		{
			name: "Clause and Reason table under another heading lists nothing",
			content: lines(
				"## Notes",
				"| Clause | Reason |",
				"|---|---|",
				"| OIDCC-NONCE-VALIDATE | 例 |",
			),
		},
		{
			name: "row whose Clause is not an ID is reported",
			content: lines(
				"## Standard Clauses Not Applied",
				"| Clause | Reason |",
				"|---|---|",
				"| nonce の照合 | 例 |",
			),
			wantFindings: []finding{{path: "d.md", line: 4, message: `Clause "nonce の照合" is not an ID`}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseMarkdown("d.md", tc.content)
			if !reflect.DeepEqual(got.notApplied, tc.want) {
				t.Errorf("notApplied = %+v, want %+v", got.notApplied, tc.want)
			}
			if !reflect.DeepEqual(got.findings, tc.wantFindings) {
				t.Errorf("findings = %+v, want %+v", got.findings, tc.wantFindings)
			}
		})
	}
}
