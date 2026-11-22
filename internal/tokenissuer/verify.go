package tokenissuer

import (
	"errors"
	"fmt"
	"time"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/githubid"
	"github.com/tmknom/gates/internal/protected"
	"github.com/tmknom/gates/internal/repository"
)

// verify は、発行結果を発行許可、許可オーナー、保護対象の一覧で照合し、INV-012 の条件をすべて満たすときだけ返却可能な発行結果を返す。
// トークンの長さと形式は、GitHub が定めておらず変わり得るため確かめない。
// 発行許可と許可オーナーのゼロ値は、対象のない発行結果や Owner ID 0 の実効対象と一致し得るが、
// それぞれ照合より先に GitHub Client の Issue と NewTokenIssuer が panic で止めるため、ここでは確かめない。
func verify(result github.IssuanceResult, grant authz.TokenGrant, allowedOwner authz.AllowedOwner, protectedRepositories protected.Repositories, now time.Time) (ReturnableIssuanceResult, error) {
	// 空のトークンは GitHub Client が読み取らないため、トークンが空でないことはこの判定に含まれる
	token, ok := result.Token()
	if !ok {
		return ReturnableIssuanceResult{}, verificationFailure(result, errors.New("token is unreadable"))
	}
	expiresAt, ok := result.ExpiresAt()
	if !ok {
		return ReturnableIssuanceResult{}, verificationFailure(result, errors.New("expires_at is unreadable"))
	}
	if !expiresAt.After(now) {
		return ReturnableIssuanceResult{}, verificationFailure(result, errors.New("expires_at is not after now"))
	}
	repositories, ok := result.EffectiveRepositories()
	if !ok {
		return ReturnableIssuanceResult{}, verificationFailure(result, errors.New("repositories are unreadable"))
	}
	if err := verifyRepositories(repositories, grant.Repositories(), allowedOwner.ID(), protectedRepositories); err != nil {
		return ReturnableIssuanceResult{}, verificationFailure(result, err)
	}
	permissions, ok := result.EffectivePermissions()
	if !ok {
		return ReturnableIssuanceResult{}, verificationFailure(result, errors.New("permissions are unreadable"))
	}
	if !permissions.EqualsExceptImplied(grant.Permissions()) {
		return ReturnableIssuanceResult{}, verificationFailure(result, errors.New("permissions do not equal token grant permissions except implied permissions"))
	}
	selection, ok := result.RepositorySelection()
	if !ok {
		return ReturnableIssuanceResult{}, verificationFailure(result, errors.New("repository_selection is unreadable"))
	}
	if selection != github.RepositorySelectionSelected {
		return ReturnableIssuanceResult{}, verificationFailure(result, errors.New("repository_selection is not selected"))
	}
	return ReturnableIssuanceResult{token: token, expiresAt: expiresAt, permissions: permissions, repositories: repositories}, nil
}

func verificationFailure(result github.IssuanceResult, condition error) error {
	return fmt.Errorf("%w: %w, github request id %q", ErrVerificationFailure, condition, result.GitHubRequestID())
}

// verifyRepositories は、実効対象と発行許可の対象が完全名で一対一に対応し、各実効対象が許可オーナーの有効なリポジトリで、保護対象と名前と ID が食い違わないかを確かめる。
// 件数が一致し、どの実効対象も未対応の対象のちょうど一つに対応すれば、余分・欠落・重複はない。
// 発行結果は照合の前の値で ID を int64 のまま持つため、Repository ID はここで ID の形式を確かめる。
// 所有オーナーの Owner ID は、ゼロ値でない許可オーナーの Owner ID と一致すれば ID の形式も満たすため、一致だけを比べる。
func verifyRepositories(repositories []github.EffectiveRepository, granted []repository.FullName, allowedOwnerID githubid.OwnerID, protectedRepositories protected.Repositories) error {
	// 件数を先に比べ、名前の総当たりを発行許可の対象の件数の上限の二乗に収める
	if len(repositories) != len(granted) {
		return errors.New("repositories count does not match token grant")
	}
	matched := make([]bool, len(granted))
	seenIDs := make(map[githubid.RepositoryID]bool, len(repositories))
	for i, effective := range repositories {
		id, err := githubid.NewRepositoryID(effective.RepositoryID())
		if err != nil {
			return fmt.Errorf("repositories[%d] id is not a positive integer", i)
		}
		if seenIDs[id] {
			return fmt.Errorf("repositories[%d] id is a duplicate", i)
		}
		seenIDs[id] = true
		if effective.OwnerID() != allowedOwnerID.Int64() {
			return fmt.Errorf("repositories[%d] owner.id does not match allowed owner id", i)
		}
		j := unmatchedGrantIndex(granted, matched, effective.FullName())
		if j < 0 {
			return fmt.Errorf("repositories[%d] full_name does not match token grant", i)
		}
		matched[j] = true
		if err := protectedRepositories.Verify(id, effective.FullName()); err != nil {
			return fmt.Errorf("repositories[%d]: %w", i, err)
		}
	}
	return nil
}

// unmatchedGrantIndex は、まだ実効対象と対応していない発行許可の対象のうち、完全名が一致するものの位置を返す。なければ -1 を返す。
// FullName は map のキーに使えないため、比較規則を repository.FullName.Equal に閉じたまま総当たりで探す。
func unmatchedGrantIndex(granted []repository.FullName, matched []bool, fullName repository.FullName) int {
	for j, name := range granted {
		if !matched[j] && name.Equal(fullName) {
			return j
		}
	}
	return -1
}
