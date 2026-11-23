package strictyaml

import (
	"github.com/tmknom/gates/internal/githubid"
	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/repository"
)

// ReadRepositoryID は、ID を引用符で囲んだ 10 進表記として読む。理由は Lambda Design の「保護対象の一覧の書式」に記述する。
func ReadRepositoryID(n Node, path string, c *Conditions) (githubid.RepositoryID, bool) {
	s, ok := readQuoted(n, path, c)
	if !ok {
		return githubid.RepositoryID{}, false
	}
	id, err := githubid.ParseRepositoryID(s)
	if err != nil {
		c.Add("%s %v", path, err)
		return githubid.RepositoryID{}, false
	}
	return id, true
}

// ReadRepositoryName は、オーナー名を含まないリポジトリ名を読む。
func ReadRepositoryName(n Node, path string, c *Conditions) (string, bool) {
	s, ok := readString(n, path, c)
	if !ok {
		return "", false
	}
	if err := repository.ValidateRepositoryName(s); err != nil {
		c.Add("%s %v", path, err)
		return "", false
	}
	return s, true
}

// ReadLimit は、権限名をキー、水準を値とする対応を、書かれた順の組として権限の上限に渡す。
// 権限名の重複も権限の上限の条件として判定させるため、キーの重複をここでは判定しない。
// YAML として読めない組があれば権限の上限を作らない。理由は Lambda Design の「保護対象の一覧の書式」に記述する。
func ReadLimit(n Node, path string, c *Conditions) permission.Limit {
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
			c.Add("%s has a value that is not a string", path)
			complete = false
			continue
		}
		entries = append(entries, permission.LimitEntry{Name: pair.key, Level: pair.value.node.Value})
	}
	if !complete {
		return permission.Limit{}
	}
	limit, err := permission.NewLimit(entries)
	if err != nil {
		c.Add("%s: %v", path, err)
	}
	return limit
}
