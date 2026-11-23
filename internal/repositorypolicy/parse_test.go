package repositorypolicy

import (
	"errors"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/allowedcaller"
)

func TestParse(t *testing.T) {
	contentsRead := mustNewPermissions(t, map[string]string{"contents": "read"})
	tests := []struct {
		name        string
		document    string
		wantVerdict allowedcaller.Verdict
	}{
		{name: "document of the format example is parsed", document: validDocument, wantVerdict: allowedcaller.Allowed},
		{name: "empty callers are parsed as allowing no caller", document: "callers: []\n", wantVerdict: allowedcaller.NoAllowedCallers},
		{name: "CRLF line breaks are accepted", document: strings.ReplaceAll(validDocument, "\n", "\r\n"), wantVerdict: allowedcaller.Allowed},
		{name: "document at the size limit is parsed", document: padded(t, validDocument, 65536), wantVerdict: allowedcaller.Allowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set, err := parse(tt.document)
			if err != nil {
				t.Fatalf("parse returned error: %v", err)
			}
			if verdict, _ := set.Authorize(mustRepositoryID(t, 100), "deploy-tools", contentsRead); verdict != tt.wantVerdict {
				t.Errorf("verdict = %v, want %v", verdict, tt.wantVerdict)
			}
		})
	}
}

//trace:checks AC-AUTHZ-015
func TestParse_Invalid(t *testing.T) {
	allowedCaller := "  - repository_id: \"100\"\n    repository_name: deploy-tools\n    permissions:\n      contents: read\n      pull_requests: write\n"
	tests := []struct {
		name     string
		document string
	}{
		// 文書
		{name: "empty document", document: ""},
		{name: "comment only document", document: "# callers: []\n"},
		{name: "null document", document: "null\n"},
		{name: "unknown version key", document: "version: \"1\"\n" + validDocument},
		{name: "duplicate callers key", document: "callers: []\n" + validDocument},
		{name: "null callers", document: "callers: ~\n"},
		{name: "multiple documents", document: validDocument + "---\ncallers: []\n"},
		// 文字と YAML の機能
		{name: "byte order mark", document: "\uFEFF" + validDocument},
		{name: "anchor", document: replaceOnce(t, validDocument, "repository_name: deploy-tools", "repository_name: &n deploy-tools")},
		{name: "explicit tag", document: replaceOnce(t, validDocument, "repository_name: deploy-tools", "repository_name: !!str deploy-tools")},
		// 許可 Caller
		{name: "unquoted id", document: replaceOnce(t, validDocument, `repository_id: "100"`, "repository_id: 100")},
		{name: "zero id", document: replaceOnce(t, validDocument, `repository_id: "100"`, `repository_id: "0"`)},
		{name: "name with owner", document: replaceOnce(t, validDocument, "repository_name: deploy-tools", "repository_name: example/deploy-tools")},
		{name: "duplicate id", document: validDocument + strings.ReplaceAll(allowedCaller, "deploy-tools", "other-tools")},
		{name: "duplicate name differing only in case", document: validDocument + strings.ReplaceAll(allowedCaller, `"100"`, `"101"`)},
		// 権限の上限
		{name: "empty permissions", document: replaceOnce(t, validDocument, "    permissions:\n      contents: read\n      pull_requests: write\n", "    permissions: {}\n")},
		{name: "permission name outside allowed permissions", document: replaceOnce(t, validDocument, "contents: read", "members: read")},
		{name: "level not defined for the permission name", document: replaceOnce(t, validDocument, "contents: read", "contents: admin")},
		{name: "duplicate permission name differing only in quotes", document: replaceOnce(t, validDocument, "pull_requests: write", "\"contents\": write")},
		// 一部だけが正しい文書
		{name: "valid allowed caller with an invalid allowed caller", document: validDocument + "  - {repository_id: \"200\", repository_name: release-tools, permissions: {}}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parse(tt.document)
			if !errors.Is(err, errInvalid) || err.Error() != "repository policy is invalid" {
				t.Errorf("parse error = %v, want %q", err, "repository policy is invalid")
			}
		})
	}
}

//trace:checks AC-AUTHZ-015
func TestParse_TooLarge(t *testing.T) {
	tests := []struct {
		name     string
		document string
	}{
		{name: "valid document over the size limit", document: padded(t, validDocument, 65537)},
		{name: "document over the size limit is rejected before reading YAML", document: padded(t, replaceOnce(t, validDocument, "deploy-tools", "!deploy-\xfftools"), 65537)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parse(tt.document)
			if !errors.Is(err, errTooLarge) || err.Error() != "repository policy is too large" {
				t.Errorf("parse error = %v, want %q", err, "repository policy is too large")
			}
		})
	}
}
