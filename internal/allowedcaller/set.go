// Package allowedcaller は、許可 Caller の集合の読み取りと判定である。
// 保護対象の一覧とリポジトリポリシーは、許可 Caller を同じ書式で書き、Caller が許可 Caller に当たるかを同じ規則で判定する（RULE-015、RULE-016）。
package allowedcaller

import (
	"fmt"
	"slices"

	"github.com/tmknom/gates/internal/githubid"
	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/repository"
	"github.com/tmknom/gates/internal/strictyaml"
)

// Set は、許可 Caller の集合である。書かれた順を保つ。ゼロ値は空の集合で、どの Caller も許さない。
type Set struct {
	callers []allowedCaller
}

// allowedCaller は、許可 Caller である。Repository ID とリポジトリ名の両方が Caller と一致するときだけ当たる。
type allowedCaller struct {
	repositoryID   githubid.RepositoryID
	repositoryName string
	limit          permission.Limit
}

var keys = []string{"repository_id", "repository_name", "permissions"}

// Read は、許可 Caller の列を読み、INV-016 と INV-017 が許可 Caller に求める条件を確かめる。
// 重複は同じ集合の中だけで判定する。保護対象の一覧で別の保護対象の許可 Caller とは比べない。
// 読めなかった値はゼロ値のまま持ち、条件があれば呼び出し元が結果を捨てる。
// 重複は総当たりで判定する。リポジトリポリシーの大きさの上限に収まる文書では、総当たりでも数十ミリ秒に収まるため。
func Read(n strictyaml.Node, path string, c *strictyaml.Conditions) Set {
	var callers []allowedCaller
	for j, item := range strictyaml.ReadSequence(n, path, c) {
		path := fmt.Sprintf("%s[%d]", path, j)
		fields := strictyaml.ReadFields(item, path, keys, c)
		if fields == nil {
			continue
		}
		id, idOK := strictyaml.ReadRepositoryID(fields["repository_id"], path+".repository_id", c)
		if idOK && slices.ContainsFunc(callers, func(a allowedCaller) bool { return a.repositoryID == id }) {
			c.Add("%s.repository_id is a duplicate", path)
		}
		name, nameOK := strictyaml.ReadRepositoryName(fields["repository_name"], path+".repository_name", c)
		if nameOK && slices.ContainsFunc(callers, func(a allowedCaller) bool { return repository.EqualFoldASCII(a.repositoryName, name) }) {
			c.Add("%s.repository_name is a duplicate", path)
		}
		limit := strictyaml.ReadLimit(fields["permissions"], path+".permissions", c)
		callers = append(callers, allowedCaller{repositoryID: id, repositoryName: name, limit: limit})
	}
	return Set{callers: callers}
}
