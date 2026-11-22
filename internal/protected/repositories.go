// Package protected は、保護対象の一覧（Protected Repositories）の読み取りと、一覧が担う発行前と発行後の判定である。
// 読み取りは INV-016 を、発行前の判定は RULE-015 を、発行後の判定は INV-012 のうち実効対象のリポジトリ名と Repository ID の対応を扱う。
package protected

import (
	"errors"

	"github.com/tmknom/gates/internal/githubid"
	"github.com/tmknom/gates/internal/permission"
)

// Repositories は、保護対象の一覧である。
// フィールドを非公開にし、ゼロ値以外は Parse でしか作れないようにする。
// 保護対象のない一覧も Parse で作る正しい値で、ゼロ値と区別する。ゼロ値を保護対象なしと読むと、初期化の漏れで保護が外れるため。
type Repositories struct {
	parsed       bool
	repositories []protectedRepository
}

// protectedRepository は、保護対象である。同一性は Repository ID で、リポジトリ名は発行前に保護対象を見分けるために持つ。
type protectedRepository struct {
	id      githubid.RepositoryID
	name    string
	callers []allowedCaller
}

// allowedCaller は、許可 Caller である。Repository ID とリポジトリ名の両方が Caller と一致するときだけ当たる。
type allowedCaller struct {
	repositoryID   githubid.RepositoryID
	repositoryName string
	limit          permission.Limit
}

// errUninitialized は、Parse を経ていないゼロ値の一覧で判定したことを表す。保護対象なしと読まずに拒否する。
var errUninitialized = errors.New("protected repositories are not initialized")

// IsZero は、Parse で作られていないゼロ値かを返す。
func (r Repositories) IsZero() bool {
	return !r.parsed
}
