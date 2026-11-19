package main

import (
	"reflect"
	"testing"
)

func TestParseActors(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name: "bold names of the items in the Actor section are Actors",
			content: lines(
				"### "+actorSection,
				"",
				"標準仕様の条項の Actor のうち、GATES が当たるものです。",
				"",
				"- **Recipient**：JWT を受け取って処理する主体",
				"- **JSON Parser**、**JWS Parser**：読み取る処理。**Library** の説明",
				"",
				"Issuer は GATES の役割ではありません。",
			),
			want: []string{"Recipient", "JSON Parser", "JWS Parser"},
		},
		{
			name:    "items outside the Actor section are not Actors",
			content: lines("### Roles", "- **Issuer**：発行する主体", "### "+actorSection, "- **Recipient**：受け取る主体", "### Next", "- **Signer**：署名する主体"),
			want:    []string{"Recipient"},
		},
		{
			name:    "items in a fenced code block are not Actors",
			content: lines("### "+actorSection, "```", "- **Issuer**：例", "```"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseActors(tc.content); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parseActors() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseIDKinds(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    []idKind
	}{
		{
			name: "rows of the ID kinds table give the prefix and its documents",
			content: lines(
				"## 1. ID Kinds",
				"",
				"| 接頭辞 | 対象 | 定めるドキュメント | ID の形 |",
				"|---|---|---|---|",
				"| `FR-` | 機能要求 | `requirements.md` | 連番（`FR-AUTHN-002`） |",
				"| `GOJ-` | 受理条件 | `gates-oidc-jwt-claims.md`、`gates-oidc-jwt-signature.md` | 名前 |",
				"| `GH-OIDC-` | GitHub Actions OIDC の条項 | `docs/external/github/oidc.md` | 名前 |",
			),
			want: []idKind{
				{prefix: "FR", documents: []string{"requirements.md"}, line: 5},
				{prefix: "GOJ", documents: []string{"gates-oidc-jwt-claims.md", "gates-oidc-jwt-signature.md"}, line: 6},
				{prefix: "GH-OIDC", documents: []string{"docs/external/github/oidc.md"}, line: 7},
			},
		},
		{
			name:    "table in a fenced code block gives nothing",
			content: lines("```", "| 接頭辞 | 定めるドキュメント |", "|---|---|", "| `FR-` | `requirements.md` |", "```"),
		},
		{
			name:    "table without the prefix column gives nothing",
			content: lines("| Clause | Reason |", "|---|---|", "| `FR-` | `requirements.md` |"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseIDKinds(tc.content); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parseIDKinds() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestIDKindDefines(t *testing.T) {
	kind := idKind{prefix: "GOJ", documents: []string{"gates-oidc-jwt-claims.md", "docs/external/github/oidc.md"}}
	cases := []struct {
		name string
		path string
		want bool
	}{
		{name: "document named by its file name defines the kind", path: "docs/design/gates-oidc-jwt-claims.md", want: true},
		{name: "document named by its path defines the kind", path: "docs/external/github/oidc.md", want: true},
		{name: "document whose name only ends with the file name does not define the kind", path: "docs/design/old-gates-oidc-jwt-claims.md"},
		{name: "document with the file name outside docs/spec and docs/design does not define the kind", path: "docs/research/gates-oidc-jwt-claims.md"},
		{name: "other document does not define the kind", path: "docs/design/domain.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := kind.defines(tc.path); got != tc.want {
				t.Errorf("defines(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}
