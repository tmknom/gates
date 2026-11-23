package allowedcaller

import (
	"testing"

	"github.com/tmknom/gates/internal/githubid"
	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/strictyaml"
)

// read は、キー callers だけを持つ文書の callers の値を Read で読み、集合と条件を返す。
func read(t *testing.T, document string) (Set, []string) {
	t.Helper()
	var c strictyaml.Conditions
	root, ok := strictyaml.Decode(document, &c)
	fields := strictyaml.ReadFields(root, "", []string{"callers"}, &c)
	if !ok || len(c.List()) > 0 {
		t.Fatalf("document %q has conditions %q before Read", document, c.List())
	}
	set := Read(fields["callers"], "callers", &c)
	return set, c.List()
}

func mustRead(t *testing.T, document string) Set {
	t.Helper()
	set, conditions := read(t, document)
	if len(conditions) > 0 {
		t.Fatalf("Read returned conditions %q", conditions)
	}
	return set
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
