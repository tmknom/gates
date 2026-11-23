package protected

import (
	"errors"
	"fmt"

	"github.com/tmknom/gates/internal/allowedcaller"
	"github.com/tmknom/gates/internal/githubid"
	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/repository"
)

// 発行前の判定の拒否の理由である。どれを返すかの区別は Lambda Design の「Audit Record」に記述する。
var (
	errNoAllowedCallers          = errors.New("no allowed callers are listed")
	errNoMatchingAllowedCaller   = errors.New("no allowed caller matches both the caller repository id and name")
	errAllowedCallerNameMismatch = errors.New("matches the caller repository id but not the name")
	errAllowedCallerIDMismatch   = errors.New("matches the caller repository name but not the id")
	errExceedsPermissionLimit    = errors.New("permissions are not within the permission limit")
)

// Authorize は、要求が RULE-015 を満たすかを判定し、満たさなければ要求の並びで最初に満たさなかった対象の理由を返す。
// Caller のリポジトリ名は文字種を満たさなくてもよく、そうした名前はどの許可 Caller にも当たらずに拒否される（INV-009）。
func (r Repositories) Authorize(callerRepositoryID githubid.RepositoryID, callerRepositoryName string, requested []repository.FullName, permissions permission.Permissions) error {
	if !r.parsed {
		return errUninitialized
	}
	for i, target := range requested {
		for j, p := range r.repositories {
			if !repository.EqualFoldASCII(target.RepositoryName(), p.name) {
				continue
			}
			if err := p.authorize(callerRepositoryID, callerRepositoryName, permissions); err != nil {
				return fmt.Errorf("repositories[%d] matches protected repositories[%d]: %w", i, j, err)
			}
		}
	}
	return nil
}

// 片方だけが一致する許可 Caller の理由の分け方は Lambda Design の「Audit Record」に記述する。
func (p protectedRepository) authorize(callerRepositoryID githubid.RepositoryID, callerRepositoryName string, permissions permission.Permissions) error {
	verdict, k := p.callers.Authorize(callerRepositoryID, callerRepositoryName, permissions)
	switch verdict {
	case allowedcaller.Allowed:
		return nil
	case allowedcaller.NoAllowedCallers:
		return errNoAllowedCallers
	case allowedcaller.ExceedsPermissionLimit:
		return fmt.Errorf("%w of callers[%d]", errExceedsPermissionLimit, k)
	case allowedcaller.NameMismatch:
		return fmt.Errorf("callers[%d] %w", k, errAllowedCallerNameMismatch)
	case allowedcaller.IDMismatch:
		return fmt.Errorf("callers[%d] %w", k, errAllowedCallerIDMismatch)
	default:
		return errNoMatchingAllowedCaller
	}
}
