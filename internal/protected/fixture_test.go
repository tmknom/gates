package protected

import (
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/githubid"
	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/repository"
)

// validDocument は、Lambda Design の書式の例と同じ形の、受理される保護対象の一覧である。
// 拒否のテストは、この文書の 1 か所だけを replaceOnce で変えて作る。
const validDocument = `repositories:
  # 共通基盤
  - id: "200"
    name: platform-infra
    callers:
      - repository_id: "100"
        repository_name: deploy-tools
        permissions:
          contents: write
          pull_requests: read
  # どの Caller にも許可しない
  - id: "201"
    name: audit-records
    callers: []
`

// replaceOnce は、document の old を 1 か所だけ new に置き換える。old がなければ、変えたつもりの条件が変わらないため失敗させる。
func replaceOnce(t *testing.T, document, old, new string) string {
	t.Helper()
	if !strings.Contains(document, old) {
		t.Fatalf("document does not contain %q", old)
	}
	return strings.Replace(document, old, new, 1)
}

func mustParse(t *testing.T, document string) Repositories {
	t.Helper()
	r, err := Parse(document)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	return r
}

func mustRepositoryID(t *testing.T, value int64) githubid.RepositoryID {
	t.Helper()
	id, err := githubid.NewRepositoryID(value)
	if err != nil {
		t.Fatalf("NewRepositoryID(%d) returned error: %v", value, err)
	}
	return id
}

func mustParseFullName(t *testing.T, s string) repository.FullName {
	t.Helper()
	f, err := repository.ParseFullName(s)
	if err != nil {
		t.Fatalf("ParseFullName(%q) returned error: %v", s, err)
	}
	return f
}

func mustNewPermissions(t *testing.T, levels map[string]string) permission.Permissions {
	t.Helper()
	p, err := permission.NewPermissions(levels)
	if err != nil {
		t.Fatalf("NewPermissions(%v) returned error: %v", levels, err)
	}
	return p
}
