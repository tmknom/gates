package protected

import (
	"fmt"
	"slices"

	yaml "go.yaml.in/yaml/v3"

	"github.com/tmknom/gates/internal/githubid"
	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/repository"
)

var (
	documentKeys            = []string{"repositories"}
	protectedRepositoryKeys = []string{"id", "name", "callers"}
	allowedCallerKeys       = []string{"repository_id", "repository_name", "permissions"}
)

// readDocument は、書式の表に沿って構文木を固定の深さでたどり、INV-016 の条件とともに保護対象の一覧を読む。
// 種類の違う値と未知のキーの値の中はたどらない。理由は Lambda Design の「保護対象の一覧の書式」に記述する。このため、たどる深さは入力によらず書式の深さに収まる。
// 読めなかった値はゼロ値のまま持ち、条件があれば呼び出し元が結果を捨てる。
// 重複は総当たりで判定する。入力は Lambda の環境変数の大きさの上限に収まるため。
func readDocument(root *yaml.Node, c *conditions) []protectedRepository {
	fields := readFields(root.Content[0], "", documentKeys, c)
	var repositories []protectedRepository
	for i, item := range readSequence(fields["repositories"], "repositories", c) {
		path := fmt.Sprintf("repositories[%d]", i)
		fields := readFields(item, path, protectedRepositoryKeys, c)
		if fields == nil {
			continue
		}
		id, idOK := readRepositoryID(fields["id"], path+".id", c)
		if idOK && slices.ContainsFunc(repositories, func(p protectedRepository) bool { return p.id == id }) {
			c.add("%s.id is a duplicate", path)
		}
		name, nameOK := readRepositoryName(fields["name"], path+".name", c)
		if nameOK && slices.ContainsFunc(repositories, func(p protectedRepository) bool { return repository.EqualFoldASCII(p.name, name) }) {
			c.add("%s.name is a duplicate", path)
		}
		callers := readAllowedCallers(fields["callers"], path+".callers", c)
		repositories = append(repositories, protectedRepository{id: id, name: name, callers: callers})
	}
	return repositories
}

// readAllowedCallers は、一つの保護対象の許可 Caller の列を読む。重複は同じ保護対象の中だけで判定し、別の保護対象の許可 Caller とは比べない。
func readAllowedCallers(n *yaml.Node, path string, c *conditions) []allowedCaller {
	var callers []allowedCaller
	for j, item := range readSequence(n, path, c) {
		path := fmt.Sprintf("%s[%d]", path, j)
		fields := readFields(item, path, allowedCallerKeys, c)
		if fields == nil {
			continue
		}
		id, idOK := readRepositoryID(fields["repository_id"], path+".repository_id", c)
		if idOK && slices.ContainsFunc(callers, func(a allowedCaller) bool { return a.repositoryID == id }) {
			c.add("%s.repository_id is a duplicate", path)
		}
		name, nameOK := readRepositoryName(fields["repository_name"], path+".repository_name", c)
		if nameOK && slices.ContainsFunc(callers, func(a allowedCaller) bool { return repository.EqualFoldASCII(a.repositoryName, name) }) {
			c.add("%s.repository_name is a duplicate", path)
		}
		limit := readLimit(fields["permissions"], path+".permissions", c)
		callers = append(callers, allowedCaller{repositoryID: id, repositoryName: name, limit: limit})
	}
	return callers
}

// readRepositoryID は、ID を引用符で囲んだ 10 進表記として読む。理由は Lambda Design の「保護対象の一覧の書式」に記述する。
func readRepositoryID(n *yaml.Node, path string, c *conditions) (githubid.RepositoryID, bool) {
	s, ok := readQuoted(n, path, c)
	if !ok {
		return githubid.RepositoryID{}, false
	}
	id, err := githubid.ParseRepositoryID(s)
	if err != nil {
		c.add("%s %v", path, err)
		return githubid.RepositoryID{}, false
	}
	return id, true
}

func readRepositoryName(n *yaml.Node, path string, c *conditions) (string, bool) {
	s, ok := readString(n, path, c)
	if !ok {
		return "", false
	}
	if err := repository.ValidateRepositoryName(s); err != nil {
		c.add("%s %v", path, err)
		return "", false
	}
	return s, true
}

// readLimit は、権限名をキー、水準を値とする対応を、書かれた順の組として権限の上限に渡す。
// 権限名の重複も権限の上限の条件として判定させるため、キーの重複をここでは判定しない。
// YAML として読めない組があれば権限の上限を作らない。理由は Lambda Design の「保護対象の一覧の書式」に記述する。
func readLimit(n *yaml.Node, path string, c *conditions) permission.Limit {
	pairs, complete, ok := readMapping(n, path, c)
	if !ok {
		return permission.Limit{}
	}
	entries := make([]permission.LimitEntry, 0, len(pairs))
	for _, pair := range pairs {
		if !accept(pair.value, c) {
			complete = false
			continue
		}
		if !isString(pair.value) {
			c.add("%s has a value that is not a string", path)
			complete = false
			continue
		}
		entries = append(entries, permission.LimitEntry{Name: pair.key, Level: pair.value.Value})
	}
	if !complete {
		return permission.Limit{}
	}
	limit, err := permission.NewLimit(entries)
	if err != nil {
		c.add("%s: %v", path, err)
	}
	return limit
}
