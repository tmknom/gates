package repositorypolicy

import (
	"github.com/tmknom/gates/internal/allowedcaller"
	"github.com/tmknom/gates/internal/githubid"
	"github.com/tmknom/gates/internal/permission"
)

// Authorize は、対象 1 件について、取得の結果が要求を許すか（RULE-016）を判定し、許さなければ理由を返す。
// 文書はここで読み取る。取得した文書を読むのは、取得する GitHub Client ではなく Authorization の側であるため（Lambda Design の「Repository Policy」）。
func (r FetchResult) Authorize(callerRepositoryID githubid.RepositoryID, callerRepositoryName string, permissions permission.Permissions) error {
	switch r.kind {
	case fetchedDocument:
		set, err := parse(r.document)
		if err != nil {
			return err
		}
		return authorize(set, callerRepositoryID, callerRepositoryName, permissions)
	case notFound:
		return errNotFound
	case notFile:
		return errNotFile
	case tooLarge:
		return errTooLarge
	default:
		return errUninitialized
	}
}

func authorize(set allowedcaller.Set, callerRepositoryID githubid.RepositoryID, callerRepositoryName string, permissions permission.Permissions) error {
	verdict, _ := set.Authorize(callerRepositoryID, callerRepositoryName, permissions)
	switch verdict {
	case allowedcaller.Allowed:
		return nil
	case allowedcaller.NoAllowedCallers:
		return errNoAllowedCallers
	case allowedcaller.ExceedsPermissionLimit:
		return errExceedsPermissionLimit
	case allowedcaller.NameMismatch:
		return errAllowedCallerNameMismatch
	case allowedcaller.IDMismatch:
		return errAllowedCallerIDMismatch
	default:
		return errNoMatchingAllowedCaller
	}
}
