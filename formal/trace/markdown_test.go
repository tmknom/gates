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
		want    []occurrence
	}{
		{
			name:    "heading starting with an ID defines it",
			content: lines("# Doc", "", "### GOJ-AUD `aud`"),
			want:    []occurrence{{id: "GOJ-AUD", path: "d.md", line: 3}},
		},
		{
			name:    "heading made of the ID alone defines it",
			content: lines("## GAJ-ISS"),
			want:    []occurrence{{id: "GAJ-ISS", path: "d.md", line: 1}},
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
				{definition: "GOJ-AUD", kind: "Satisfies", clause: "RFC7519-AUD-REJECT", path: "d.md", line: 5},
				{definition: "GOJ-AUD", kind: "Possibly Rejects", clause: "GH-OIDC-AUD-X", path: "d.md", line: 6},
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
				{definition: "GOJ-IAT", kind: "None", clause: "なし", path: "d.md", line: 5},
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
				{definition: "GOJ-AUD", kind: "Satisfies", clause: "RFC7519-AUD-REJECT", path: "d.md", line: 4},
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
