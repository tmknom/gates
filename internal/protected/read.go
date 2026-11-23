package protected

import (
	"fmt"
	"slices"

	"github.com/tmknom/gates/internal/allowedcaller"
	"github.com/tmknom/gates/internal/repository"
	"github.com/tmknom/gates/internal/strictyaml"
)

var (
	documentKeys            = []string{"repositories"}
	protectedRepositoryKeys = []string{"id", "name", "callers"}
)

// readDocument は、書式の表に沿って構文木を固定の深さでたどり、INV-016 の条件とともに保護対象の一覧を読む。
// 種類の違う値と未知のキーの値の中はたどらない。理由は Lambda Design の「保護対象の一覧の書式」に記述する。このため、たどる深さは入力によらず書式の深さに収まる。
// 読めなかった値はゼロ値のまま持ち、条件があれば呼び出し元が結果を捨てる。
// 重複は総当たりで判定する。入力は Lambda の環境変数の大きさの上限に収まるため。
func readDocument(root strictyaml.Node, c *strictyaml.Conditions) []protectedRepository {
	fields := strictyaml.ReadFields(root, "", documentKeys, c)
	var repositories []protectedRepository
	for i, item := range strictyaml.ReadSequence(fields["repositories"], "repositories", c) {
		path := fmt.Sprintf("repositories[%d]", i)
		fields := strictyaml.ReadFields(item, path, protectedRepositoryKeys, c)
		if fields == nil {
			continue
		}
		id, idOK := strictyaml.ReadRepositoryID(fields["id"], path+".id", c)
		if idOK && slices.ContainsFunc(repositories, func(p protectedRepository) bool { return p.id == id }) {
			c.Add("%s.id is a duplicate", path)
		}
		name, nameOK := strictyaml.ReadRepositoryName(fields["name"], path+".name", c)
		if nameOK && slices.ContainsFunc(repositories, func(p protectedRepository) bool { return repository.EqualFoldASCII(p.name, name) }) {
			c.Add("%s.name is a duplicate", path)
		}
		callers := allowedcaller.Read(fields["callers"], path+".callers", c)
		repositories = append(repositories, protectedRepository{id: id, name: name, callers: callers})
	}
	return repositories
}
