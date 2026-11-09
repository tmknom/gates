package request_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/credential"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/request"
)

const jwt = "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJyZXBvIn0.c2lnbmF0dXJl"

// payloadOf は、入力を Parse し、成功を前提に Payload を文字列で返す。
func payloadOf(t *testing.T, repositories, permissions string) string {
	t.Helper()

	tokenRequest, err := request.Parse(repositories, permissions)
	if err != nil {
		t.Fatalf("Parse(%q, %q) error = %v", repositories, permissions, err)
	}
	return string(tokenRequest.Payload(credential.NewOIDCJWT(jwt)))
}

func payload(repositories, permissions string) string {
	return `{"version":1,"id_token":"` + jwt + `","repositories":` + repositories + `,"permissions":` + permissions + `}`
}

func TestParse_Payload(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		repositories string
		permissions  string
		want         string
	}{
		"one repository and one permission": {
			"example-org/repo-a", "contents:read",
			payload(`["example-org/repo-a"]`, `{"contents":"read"}`),
		},
		"two repositories keep input order": {
			"example-org/repo-b,example-org/repo-a", "contents:read",
			payload(`["example-org/repo-b","example-org/repo-a"]`, `{"contents":"read"}`),
		},
		"space, tab, CR and LF around elements are removed": {
			" example-org/repo-a ,\texample-org/repo-b\r\n", "\r\n contents:read\t",
			payload(`["example-org/repo-a","example-org/repo-b"]`, `{"contents":"read"}`),
		},
		"non-breaking space is not removed": {
			" example-org/repo-a", "contents:read",
			payload("[\" example-org/repo-a\"]", `{"contents":"read"}`),
		},
		"two permissions keep input order": {
			"example-org/repo-a", "metadata:read,contents:read",
			payload(`["example-org/repo-a"]`, `{"metadata":"read","contents":"read"}`),
		},
		"permission names differing only in case are both kept": {
			"example-org/repo-a", "contents:read,Contents:read",
			payload(`["example-org/repo-a"]`, `{"contents":"read","Contents":"read"}`),
		},
		"repositories differing only in non-ASCII case are not duplicates": {
			"example-org/repo-ſ,example-org/repo-s", "contents:read",
			payload(`["example-org/repo-ſ","example-org/repo-s"]`, `{"contents":"read"}`),
		},
		"repository that is a prefix of another is not a duplicate": {
			"example-org/repo-a,example-org/repo-ab", "contents:read",
			payload(`["example-org/repo-a","example-org/repo-ab"]`, `{"contents":"read"}`),
		},
		"character classes are not validated": {
			"example_org/repo.a", "contents:admin",
			payload(`["example_org/repo.a"]`, `{"contents":"admin"}`),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := payloadOf(t, tt.repositories, tt.permissions); got != tt.want {
				t.Errorf("Payload() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestParse_Invalid(t *testing.T) {
	t.Parallel()

	const (
		validRepositories = "example-org/repo-a"
		validPermissions  = "contents:read"
	)

	tests := map[string]struct {
		repositories string
		permissions  string
		want         []string
	}{
		"empty element names its position": {
			"example-org/repo-a,,example-org/repo-b", validPermissions,
			[]string{"repositories: element 2 is empty"},
		},
		"trailing comma is an empty element": {
			"example-org/repo-a,", validPermissions,
			[]string{"repositories: element 2 is empty"},
		},
		"whitespace-only element is empty": {
			validRepositories, "contents:read, ",
			[]string{"permissions: element 2 is empty"},
		},
		"empty input is element 1 empty": {
			"", validPermissions,
			[]string{"repositories: element 1 is empty"},
		},
		"uppercase Z and lowercase z are duplicates": {
			"example-org/repo-Z,example-org/repo-z", validPermissions,
			[]string{`repositories: "example-org/repo-Z", "example-org/repo-z" are duplicates`},
		},
		"ASCII case-insensitive duplicates show both elements": {
			"example-org/repo-a,Example-Org/Repo-A", validPermissions,
			[]string{`repositories: "example-org/repo-a", "Example-Org/Repo-A" are duplicates (compared case-insensitively)`},
		},
		"repository without slash": {
			"example-org", validPermissions,
			[]string{`repositories: "example-org" is not in the form owner/repo`},
		},
		"repository with two slashes": {
			"a/b/c", validPermissions,
			[]string{`repositories: "a/b/c" is not in the form owner/repo`},
		},
		"empty owner": {
			"/repo-a", validPermissions,
			[]string{`repositories: "/repo-a" is not in the form owner/repo`},
		},
		"empty repository name": {
			"example-org/", validPermissions,
			[]string{`repositories: "example-org/" is not in the form owner/repo`},
		},
		"space before the slash is rejected without trimming": {
			"example-org /repo-a", validPermissions,
			[]string{`repositories: "example-org /repo-a" is not in the form owner/repo`},
		},
		"space after the slash is rejected without trimming": {
			"example-org/ repo-a", validPermissions,
			[]string{`repositories: "example-org/ repo-a" is not in the form owner/repo`},
		},
		"permission without colon": {
			validRepositories, "contents",
			[]string{`permissions: "contents" is not in the form name:level`},
		},
		"empty level": {
			validRepositories, "contents:",
			[]string{`permissions: "contents:" is not in the form name:level`},
		},
		"empty permission name": {
			validRepositories, ":read",
			[]string{`permissions: ":read" is not in the form name:level`},
		},
		"two colons": {
			validRepositories, "a:b:c",
			[]string{`permissions: "a:b:c" is not in the form name:level`},
		},
		"space before the colon": {
			validRepositories, "contents :write",
			[]string{`permissions: "contents :write" is not in the form name:level`},
		},
		"space after the colon": {
			validRepositories, "contents: write",
			[]string{`permissions: "contents: write" is not in the form name:level`},
		},
		"duplicate permission names show both elements": {
			validRepositories, "contents:read,contents:write",
			[]string{`permissions: "contents:read", "contents:write" have the same permission name`},
		},
		"invalid UTF-8 element is rejected and shown quoted": {
			"example-org/repo-\xff", validPermissions,
			[]string{`repositories: "example-org/repo-\xff" is not valid UTF-8`},
		},
		"invalid UTF-8 permission is rejected and shown quoted": {
			validRepositories, "contents:\xff",
			[]string{`permissions: "contents:\xff" is not valid UTF-8`},
		},
		"control characters in elements are escaped": {
			"example-org\n::add-mask::x", validPermissions,
			[]string{`"example-org\n::add-mask::x"`},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := request.Parse(tt.repositories, tt.permissions)

			if err == nil {
				t.Fatal("Parse() error = nil, want an error")
			}
			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("Parse() error = %q, want it to contain %q", err.Error(), want)
				}
			}
		})
	}
}

func TestParse_InvalidReportsEveryErrorOnItsOwnLine(t *testing.T) {
	t.Parallel()

	_, err := request.Parse("example-org,,a/b/c", "contents")

	if err == nil {
		t.Fatal("Parse() error = nil, want an error")
	}
	want := []string{
		`permissions: "contents" is not in the form name:level`,
		`repositories: "a/b/c" is not in the form owner/repo`,
		`repositories: "example-org" is not in the form owner/repo`,
		"repositories: element 2 is empty",
	}
	got := strings.Split(err.Error(), "\n")
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("Parse() error lines = %q, want %q in any order", got, want)
	}
}
