package authz

import (
	"fmt"

	"github.com/tmknom/gates/internal/caller"
	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/protected"
	"github.com/tmknom/gates/internal/request"
)

// Grant は、発行上限と保護対象の条件を含む認可条件と Installation の有効性（RULE-010、RULE-011）を満たすときだけ TokenGrant を確定する。
// Authorize を通した要求にも認可条件を判定し直す理由と、両方を満たさないときに認可拒否を返す理由は、Lambda Design の「Processing Flow」に記述する。
func Grant(c caller.Caller, req request.TokenRequest, allowedOwner AllowedOwner, issuablePermissions permission.IssuablePermissions, protectedRepositories protected.Repositories, inst Installation) (TokenGrant, error) {
	if err := Authorize(c, req, allowedOwner, issuablePermissions, protectedRepositories); err != nil {
		return TokenGrant{}, err
	}
	if inst.installationOwnerID != allowedOwner.ID() {
		return TokenGrant{}, fmt.Errorf("%w: installation owner id does not match allowed owner id", ErrInstallationUnavailable)
	}
	if inst.suspended {
		return TokenGrant{}, fmt.Errorf("%w: installation is suspended", ErrInstallationUnavailable)
	}
	return TokenGrant{
		installationID: inst.id,
		repositories:   req.Repositories(),
		permissions:    req.Permissions(),
	}, nil
}
