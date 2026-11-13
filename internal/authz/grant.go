package authz

import (
	"fmt"

	"github.com/tmknom/gates/internal/caller"
	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/request"
)

// Grant は、認可条件と Installation の有効性（RULE-010、RULE-011）を満たすときだけ TokenGrant を確定する。
// Authorize を通した要求にも認可条件を判定し直す理由と、両方を満たさないときに認可拒否を返す理由は、Lambda Design の「Processing Flow」に記述する。
func Grant(c caller.Caller, req request.TokenRequest, owner AllowedOwner, inst Installation) (TokenGrant, error) {
	if err := Authorize(c, req, owner); err != nil {
		return TokenGrant{}, err
	}
	if inst.installationOwnerID != owner.ID() {
		return TokenGrant{}, fmt.Errorf("%w: installation owner id does not match allowed owner id", ErrInstallationUnavailable)
	}
	if !inst.suspendedAt.IsZero() {
		return TokenGrant{}, fmt.Errorf("%w: installation is suspended", ErrInstallationUnavailable)
	}
	// 要求の Permissions を写さない。認可条件で両者が等しいことは確かめているが、写すと、認可の条件を将来変えたときに要求の値がそのまま発行範囲に流れるため
	return TokenGrant{
		installationID: inst.id,
		repositories:   req.Repositories(),
		permissions:    permission.AllowedPermissions(),
	}, nil
}
