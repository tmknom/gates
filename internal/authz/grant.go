package authz

import (
	"fmt"

	"github.com/tmknom/gates/internal/caller"
	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/protected"
	"github.com/tmknom/gates/internal/repositorypolicy"
	"github.com/tmknom/gates/internal/request"
)

// Grant は、リポジトリポリシーを適用しないデプロイで、発行上限と保護対象の条件を含む認可条件と Installation の有効性（RULE-010、RULE-011）を満たすときだけ TokenGrant を確定する。
// Authorize を通した要求にも認可条件を判定し直す理由と、両方を満たさないときに認可拒否を返す理由は、Lambda Design の「Processing Flow」に記述する。
// 適用するデプロイでは拒否する。リポジトリポリシーの条件を判定しない発行許可を確定させないためである（INV-011）。
func Grant(c caller.Caller, req request.TokenRequest, allowedOwner AllowedOwner, issuablePermissions permission.IssuablePermissions, protectedRepositories protected.Repositories, enforcement RepositoryPolicyEnforcement, inst Installation) (TokenGrant, error) {
	if err := Authorize(c, req, allowedOwner, issuablePermissions, protectedRepositories, enforcement); err != nil {
		return TokenGrant{}, err
	}
	if enforcement != RepositoryPolicyNotEnforced {
		return TokenGrant{}, fmt.Errorf("%w: token grant requires repository policies under repository policy enforcement", ErrAuthorizationDenied)
	}
	if err := inst.verifyUsableBy(allowedOwner); err != nil {
		return TokenGrant{}, err
	}
	return newTokenGrant(inst, req), nil
}

// GrantWithRepositoryPolicies は、リポジトリポリシーを適用するデプロイで、Grant の条件に加えて各対象のリポジトリポリシーの条件（RULE-016）を満たすときだけ TokenGrant を確定する。
// fetchResults は要求の対象と同じ順に並べる。件数が違えば、判定していない対象が残るか、どの対象の結果かが分からないため拒否する。
// 要求の対象の順で最初に満たさない対象の位置を返すのは、監査記録がその対象を示すためである（Lambda Design の「Processing Flow」）。
func GrantWithRepositoryPolicies(c caller.Caller, req request.TokenRequest, allowedOwner AllowedOwner, issuablePermissions permission.IssuablePermissions, protectedRepositories protected.Repositories, enforcement RepositoryPolicyEnforcement, inst Installation, fetchResults []repositorypolicy.FetchResult) (TokenGrant, error) {
	if err := Authorize(c, req, allowedOwner, issuablePermissions, protectedRepositories, enforcement); err != nil {
		return TokenGrant{}, err
	}
	if enforcement != RepositoryPolicyEnforced {
		return TokenGrant{}, fmt.Errorf("%w: repository policies are not used without repository policy enforcement", ErrAuthorizationDenied)
	}
	if err := inst.verifyUsableBy(allowedOwner); err != nil {
		return TokenGrant{}, err
	}
	if len(fetchResults) != len(req.Repositories()) {
		return TokenGrant{}, fmt.Errorf("%w: repository policy fetch results count does not match repositories", ErrAuthorizationDenied)
	}
	for i, fetchResult := range fetchResults {
		if err := fetchResult.Authorize(c.RepositoryID(), c.RepositoryName(), req.Permissions()); err != nil {
			return TokenGrant{}, fmt.Errorf("%w: repositories[%d]: %w", ErrAuthorizationDenied, i, err)
		}
	}
	return newTokenGrant(inst, req), nil
}

func newTokenGrant(inst Installation, req request.TokenRequest) TokenGrant {
	return TokenGrant{
		installationID: inst.id,
		repositories:   req.Repositories(),
		permissions:    req.Permissions(),
	}
}
