package issuer

import (
	"errors"
	"fmt"
	"time"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/repository"
)

// Verify は、発行結果を発行許可と許可オーナーで照合し、Domain Model の「Invariants」が定める返却可能な発行結果の条件をすべて満たすときだけ InstallationToken を返す。
// トークンの長さと形式は、GitHub が定めておらず変わり得るため確かめない。
func Verify(result github.IssuanceResult, grant authz.TokenGrant, owner authz.AllowedOwner, now time.Time) (InstallationToken, error) {
	if grant.IsZero() {
		// ゼロ値は対象を持たず、対象のない発行結果が件数の一致を満たしてしまうため照合しない
		panic("token grant is not created by Grant")
	}
	if owner.IsZero() {
		// ゼロ値の Owner ID 0 は、応答が Owner ID 0 を返した実効対象と一致し得るため照合しない
		panic("allowed owner is not created by NewAllowedOwner")
	}

	// 空のトークンは GitHub Client が読み取らないため、トークンが空でないことはこの判定に含まれる
	token, ok := result.Token()
	if !ok {
		return InstallationToken{}, verificationFailure(result, errors.New("token is unreadable"))
	}
	expiresAt, ok := result.ExpiresAt()
	if !ok {
		return InstallationToken{}, verificationFailure(result, errors.New("expires_at is unreadable"))
	}
	if !expiresAt.After(now) {
		return InstallationToken{}, verificationFailure(result, errors.New("expires_at is not after now"))
	}
	repositories, ok := result.EffectiveRepositories()
	if !ok {
		return InstallationToken{}, verificationFailure(result, errors.New("repositories are unreadable"))
	}
	if err := verifyRepositories(repositories, grant.Repositories(), owner.ID()); err != nil {
		return InstallationToken{}, verificationFailure(result, err)
	}
	permissions, ok := result.EffectivePermissions()
	if !ok {
		return InstallationToken{}, verificationFailure(result, errors.New("permissions are unreadable"))
	}
	if !permissions.EqualsAllowedExceptImplied() {
		return InstallationToken{}, verificationFailure(result, errors.New("permissions do not equal allowed permissions except implied permissions"))
	}
	selection, ok := result.RepositorySelection()
	if !ok {
		return InstallationToken{}, verificationFailure(result, errors.New("repository_selection is unreadable"))
	}
	if selection != github.RepositorySelectionSelected {
		return InstallationToken{}, verificationFailure(result, errors.New("repository_selection is not selected"))
	}
	return InstallationToken{token: token, expiresAt: expiresAt, permissions: permissions, repositories: repositories}, nil
}

func verificationFailure(result github.IssuanceResult, condition error) error {
	return fmt.Errorf("%w: %w, request id %q", ErrVerificationFailure, condition, result.RequestID())
}

// verifyRepositories は、実効対象と発行許可の対象が完全名で一対一に対応し、各実効対象が許可オーナーの有効なリポジトリであるかを確かめる。
// 件数が一致し、どの実効対象も未対応の対象のちょうど一つに対応すれば、余分・欠落・重複はない。
func verifyRepositories(repositories []github.EffectiveRepository, granted []repository.FullName, ownerID int64) error {
	// 件数を先に比べ、名前の総当たりを発行許可の対象の件数の上限の二乗に収める
	if len(repositories) != len(granted) {
		return errors.New("repositories count does not match token grant")
	}
	matched := make([]bool, len(granted))
	seenIDs := make(map[int64]bool, len(repositories))
	for i, effective := range repositories {
		id := effective.RepositoryID()
		if id <= 0 {
			return fmt.Errorf("repositories[%d] id is not a positive integer", i)
		}
		if seenIDs[id] {
			return fmt.Errorf("repositories[%d] id is a duplicate", i)
		}
		seenIDs[id] = true
		if effective.OwnerID() != ownerID {
			return fmt.Errorf("repositories[%d] owner.id does not match allowed owner id", i)
		}
		j := unmatchedGrantIndex(granted, matched, effective.FullName())
		if j < 0 {
			return fmt.Errorf("repositories[%d] full_name does not match token grant", i)
		}
		matched[j] = true
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
