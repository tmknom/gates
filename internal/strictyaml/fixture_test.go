package strictyaml

import (
	"testing"

	"github.com/tmknom/gates/internal/permission"
)

// value は、トップレベルが対応の文書を構文木に復号し、キー v の値の節を返す。
// 読み取りの関数を通さずに節を取り出し、テストする関数のほかの規則が結果に混ざらないようにする。
func value(t *testing.T, document string) Node {
	t.Helper()
	var c Conditions
	root, ok := Decode(document, &c)
	if !ok || len(c.List()) > 0 {
		t.Fatalf("Decode(%q) = %v with conditions %q, want true without conditions", document, ok, c.List())
	}
	for i := 0; i < len(root.node.Content); i += 2 {
		if root.node.Content[i].Value == "v" {
			return Node{node: root.node.Content[i+1]}
		}
	}
	t.Fatalf("document %q has no key v", document)
	return Node{}
}

func mustNewPermissions(t *testing.T, levels map[string]string) permission.Permissions {
	t.Helper()
	p, err := permission.NewPermissions(levels)
	if err != nil {
		t.Fatalf("NewPermissions(%v) returned error: %v", levels, err)
	}
	return p
}
