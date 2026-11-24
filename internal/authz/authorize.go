package authz

import (
	"fmt"

	"github.com/tmknom/gates/internal/caller"
	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/protected"
	"github.com/tmknom/gates/internal/request"
)

// maxRepositoriesUnderEnforcement は、リポジトリポリシーを適用するデプロイでの対象の件数の上限である。値の根拠は Lambda Design の「対象の件数の上限」に記述する。
const maxRepositoriesUnderEnforcement = 10

// Authorize は、Security Design の認可条件を番号順に判定し、リポジトリポリシーを適用するデプロイでは続けて対象の件数の上限を判定する。
// 最初に満たさなかった条件で返す。ほかの条件の成否は、監査記録が別の項目として持つ Caller と要求から判定できるため。
// 水準の包含は許可権限に含まれることを条件に持つため発行上限の判定だけで足りるが、許可権限も別の条件として判定する。理由は RULE-010 の根拠に記述する。
// 保護対象の一覧はリポジトリ名だけで保護対象を見分けるため、オーナー名の条件を通った要求だけを渡す。
// リポジトリポリシーの条件（RULE-016）は、取得したリポジトリポリシーを要するため判定しない。
func Authorize(c caller.Caller, req request.TokenRequest, allowedOwner AllowedOwner, issuablePermissions permission.IssuablePermissions, protectedRepositories protected.Repositories, enforcement RepositoryPolicyEnforcement) error {
	if c.OwnerID() != allowedOwner.ID() {
		return fmt.Errorf("%w: caller owner id does not match allowed owner id", ErrAuthorizationDenied)
	}
	for i, fullName := range req.Repositories() {
		if !fullName.OwnerName().Equal(allowedOwner.Name()) {
			return fmt.Errorf("%w: repositories[%d] owner name does not match allowed owner name", ErrAuthorizationDenied, i)
		}
	}
	if !req.Permissions().WithinAllowed() {
		return fmt.Errorf("%w: permissions are not within allowed permissions", ErrAuthorizationDenied)
	}
	if !issuablePermissions.Contains(req.Permissions()) {
		return fmt.Errorf("%w: permissions are not within issuable permissions", ErrAuthorizationDenied)
	}
	if err := protectedRepositories.Authorize(c.RepositoryID(), c.RepositoryName(), req.Repositories(), req.Permissions()); err != nil {
		return fmt.Errorf("%w: %w", ErrAuthorizationDenied, err)
	}
	switch enforcement {
	case RepositoryPolicyNotEnforced:
		return nil
	case RepositoryPolicyEnforced:
		if len(req.Repositories()) > maxRepositoriesUnderEnforcement {
			return fmt.Errorf("%w: repositories count exceeds the limit under repository policy enforcement", ErrAuthorizationDenied)
		}
		return nil
	default:
		return fmt.Errorf("%w: repository policy enforcement is not initialized", ErrAuthorizationDenied)
	}
}
