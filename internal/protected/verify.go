package protected

import (
	"errors"
	"fmt"

	"github.com/tmknom/gates/internal/githubid"
	"github.com/tmknom/gates/internal/repository"
)

// 発行後の判定の拒否の理由である。両者を分ける理由は Lambda Design の「Audit Record」に記述する。
var (
	errProtectedIDMismatch   = errors.New("matches the repository name but not the repository id")
	errProtectedNameMismatch = errors.New("matches the repository id but not the repository name")
	// errIncompleteEffectiveRepository は、実効対象の Repository ID か完全名がゼロ値であることを表す。対応を確かめられないため、食い違いがないとは読まない
	errIncompleteEffectiveRepository = errors.New("effective repository id and full name must be set")
)

// Verify は、実効対象と各保護対象について、リポジトリ名の一致と Repository ID の一致が同値であるか（INV-012）を判定し、一覧の並びで最初の食い違いを返す。
// 名前が一致した保護対象だけでなくすべての保護対象と比べる理由は Lambda Design の「Processing Flow」に記述する。
// 実効対象のオーナーは、所有オーナーの Owner ID の照合が確かめるため、ここではリポジトリ名だけを比べる。
func (r Repositories) Verify(effectiveRepositoryID githubid.RepositoryID, effectiveFullName repository.FullName) error {
	if !r.parsed {
		return errUninitialized
	}
	if effectiveRepositoryID == (githubid.RepositoryID{}) || effectiveFullName.RepositoryName() == "" {
		return errIncompleteEffectiveRepository
	}
	for j, p := range r.repositories {
		nameMatches := repository.EqualFoldASCII(effectiveFullName.RepositoryName(), p.name)
		idMatches := effectiveRepositoryID == p.id
		switch {
		case nameMatches && !idMatches:
			return fmt.Errorf("protected repositories[%d] %w", j, errProtectedIDMismatch)
		case idMatches && !nameMatches:
			return fmt.Errorf("protected repositories[%d] %w", j, errProtectedNameMismatch)
		}
	}
	return nil
}
