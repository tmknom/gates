package repositorypolicy

import (
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/githubid"
	"github.com/tmknom/gates/internal/permission"
)

// validDocument は、Lambda Design の書式の例と同じ形の、有効なリポジトリポリシーである。
const validDocument = `callers:
  # 共通のビルド基盤
  - repository_id: "100"
    repository_name: deploy-tools
    permissions:
      contents: read
      pull_requests: write
`

// replaceOnce は、document の old を 1 か所だけ new に置き換える。old がなければ、変えたつもりの条件が変わらないため失敗させる。
func replaceOnce(t *testing.T, document, old, new string) string {
	t.Helper()
	if !strings.Contains(document, old) {
		t.Fatalf("document does not contain %q", old)
	}
	return strings.Replace(document, old, new, 1)
}

// padded は、document の末尾にコメントの行を足し、ちょうど size バイトの文書にする。
func padded(t *testing.T, document string, size int) string {
	t.Helper()
	padding := size - len(document) - len("#\n")
	if padding < 0 {
		t.Fatalf("document of %d bytes does not fit in %d bytes", len(document), size)
	}
	return document + "#" + strings.Repeat("x", padding) + "\n"
}

func mustRepositoryID(t *testing.T, value int64) githubid.RepositoryID {
	t.Helper()
	id, err := githubid.NewRepositoryID(value)
	if err != nil {
		t.Fatalf("NewRepositoryID(%d) returned error: %v", value, err)
	}
	return id
}

func mustNewPermissions(t *testing.T, levels map[string]string) permission.Permissions {
	t.Helper()
	p, err := permission.NewPermissions(levels)
	if err != nil {
		t.Fatalf("NewPermissions(%v) returned error: %v", levels, err)
	}
	return p
}
