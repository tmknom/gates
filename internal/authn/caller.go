package authn

import (
	"fmt"

	"github.com/tmknom/gates/internal/caller"
)

// callerFromClaims は、検証済みの GATES 用 OIDC JWT の Claim から Caller を確定する。
// Claim を JSON のまま受けるのは、JSON の型の規則と ID の表記の規則を、この関数の 1 経路で判定するため。
func callerFromClaims(rawClaims []byte) (caller.Caller, error) {
	c, err := decodeClaims(rawClaims)
	if err != nil {
		return caller.Caller{}, err
	}

	ownerID, err := parseID("repository_owner_id", c.RepositoryOwnerID)
	if err != nil {
		return caller.Caller{}, err
	}
	repositoryID, err := parseID("repository_id", c.RepositoryID)
	if err != nil {
		return caller.Caller{}, err
	}
	runID, err := parseID("run_id", c.RunID)
	if err != nil {
		return caller.Caller{}, err
	}
	runAttempt, err := parseID("run_attempt", c.RunAttempt)
	if err != nil {
		return caller.Caller{}, err
	}

	subjectOwnerID, subjectRepositoryID, err := c.Sub.parse()
	if err != nil {
		return caller.Caller{}, err
	}
	if subjectOwnerID != ownerID {
		return caller.Caller{}, fmt.Errorf("%w: sub owner id does not match repository_owner_id", ErrAuthenticationFailure)
	}
	if subjectRepositoryID != repositoryID {
		return caller.Caller{}, fmt.Errorf("%w: sub repository id does not match repository_id", ErrAuthenticationFailure)
	}

	verified, err := caller.NewCaller(ownerID, repositoryID, runID, runAttempt, c.triggerAttributes())
	if err != nil {
		return caller.Caller{}, fmt.Errorf("%w: %w", ErrAuthenticationFailure, err)
	}
	return verified, nil
}
