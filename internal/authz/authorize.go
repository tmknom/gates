package authz

import (
	"fmt"

	"github.com/tmknom/gates/internal/caller"
	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/request"
)

// Authorize は、Security Design の認可条件を番号順に判定する。
// 最初に満たさなかった条件で返す。ほかの条件の成否は、監査記録が別の項目として持つ Caller と要求から判定できるため。
// 水準の包含は許可権限に含まれることを条件に持つため発行上限の判定だけで足りるが、許可権限も別の条件として判定する。理由は RULE-010 の根拠に記述する。
func Authorize(c caller.Caller, req request.TokenRequest, allowedOwner AllowedOwner, issuablePermissions permission.IssuablePermissions) error {
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
	return nil
}
