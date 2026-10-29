package authz

import (
	"fmt"

	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/request"
)

// Authorize は、Security Design の認可条件を番号順に判定する。
// 最初に満たさなかった条件で返す。ほかの条件の成否は、監査記録が別の項目として持つ Caller と要求から判定できるため。
func Authorize(caller Caller, req request.TokenRequest, owner AllowedOwner) error {
	if caller.OwnerID() != owner.ID() {
		return fmt.Errorf("%w: caller owner id does not match allowed owner id", ErrAuthorizationDenied)
	}
	for i, fullName := range req.Repositories() {
		if !fullName.OwnerName().Equal(owner.Name()) {
			return fmt.Errorf("%w: repositories[%d] owner name does not match allowed owner name", ErrAuthorizationDenied, i)
		}
	}
	if !req.Permissions().Equal(permission.AllowedPermissions()) {
		return fmt.Errorf("%w: app permissions are not equal to allowed permissions", ErrAuthorizationDenied)
	}
	return nil
}
