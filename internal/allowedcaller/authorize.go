package allowedcaller

import (
	"github.com/tmknom/gates/internal/githubid"
	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/repository"
)

// Verdict は、許可 Caller の判定結果である。Allowed だけが要求を許す。
// 誤りではなく判定結果を返すのは、拒否の理由の文言と示す位置が、保護対象の一覧とリポジトリポリシーで異なるため。
type Verdict int

const (
	// NoAllowedCallers は、集合に許可 Caller がないことを表す。
	NoAllowedCallers Verdict = iota
	// NoMatchingAllowedCaller は、Repository ID とリポジトリ名のどちらかでも Caller と一致する許可 Caller がないことを表す。
	NoMatchingAllowedCaller
	// NameMismatch は、Repository ID だけが Caller と一致する許可 Caller があることを表す。
	NameMismatch
	// IDMismatch は、リポジトリ名だけが Caller と一致する許可 Caller があることを表す。
	IDMismatch
	// ExceedsPermissionLimit は、Caller が当たる許可 Caller の権限の上限に、要求の Permissions が含まれないことを表す。
	ExceedsPermissionLimit
	// Allowed は、Caller が当たる許可 Caller の権限の上限に、要求の Permissions が含まれることを表す。
	Allowed
)

// Authorize は、Repository ID とリポジトリ名の両方が Caller と一致する許可 Caller を探し、その権限の上限に要求を照合する。
// 判定結果に関わった許可 Caller の、書かれた順の位置を添える。関わった許可 Caller がなければ -1 を添える。
// 片方だけが一致する許可 Caller の判定結果の分け方は Domain Model の Protected Repositories に記述する。
// Caller のリポジトリ名は文字種を満たさなくてもよく、そうした名前はどの許可 Caller にも一致しない（INV-009）。
// 同じ集合の中で許可 Caller の ID と名前は重複しないため（INV-016、INV-017）、どちらの一致も高々一つである。
func (s Set) Authorize(callerRepositoryID githubid.RepositoryID, callerRepositoryName string, permissions permission.Permissions) (Verdict, int) {
	if len(s.callers) == 0 {
		return NoAllowedCallers, -1
	}
	idOnly, nameOnly := -1, -1
	for k, a := range s.callers {
		idMatches := a.repositoryID == callerRepositoryID
		nameMatches := repository.EqualFoldASCII(a.repositoryName, callerRepositoryName)
		switch {
		case idMatches && nameMatches:
			if !a.limit.Contains(permissions) {
				return ExceedsPermissionLimit, k
			}
			return Allowed, k
		case idMatches:
			idOnly = k
		case nameMatches:
			nameOnly = k
		}
	}
	switch {
	case idOnly >= 0:
		return NameMismatch, idOnly
	case nameOnly >= 0:
		return IDMismatch, nameOnly
	}
	return NoMatchingAllowedCaller, -1
}
