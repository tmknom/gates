package tokenissuer

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/githubid"
	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/protected"
	"github.com/tmknom/gates/internal/repository"
	"github.com/tmknom/gates/internal/repositorypolicy"
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
	terms := grantTerms{name: "token grant", repositories: grant.Repositories(), permissions: grant.Permissions()}
	verified, err := verifyTerms(result, terms, allowedOwner, protectedRepositories, now)
	if err != nil {
		return ReturnableIssuanceResult{}, verificationFailure(result, err)
	}
	return ReturnableIssuanceResult{token: token, expiresAt: verified.expiresAt, permissions: verified.permissions, repositories: verified.repositories}, nil
}

// verifyPolicyReadToken は、発行結果をポリシー読み取り許可、許可オーナー、保護対象の一覧で照合し、INV-019 の条件をすべて満たすときだけ照合済みのポリシー読み取りトークンを返す。
// 利用者に返すトークンの照合と同じ条件を同じコードで確かめ、読めるファイルのパスの条件を加える。
// ポリシー読み取り許可のゼロ値は、照合より先に GitHub Client の IssuePolicyReadToken が panic で止める。
func verifyPolicyReadToken(result github.IssuanceResult, grant authz.PolicyReadGrant, allowedOwner authz.AllowedOwner, protectedRepositories protected.Repositories, now time.Time) (verifiedPolicyReadToken, error) {
	token, ok := result.PolicyReadToken()
	if !ok {
		return verifiedPolicyReadToken{}, verificationFailure(result, errors.New("token is unreadable"))
	}
	granted := grant.Repositories()
	terms := grantTerms{name: "policy read grant", repositories: granted, permissions: grant.Permissions()}
	verified, err := verifyTerms(result, terms, allowedOwner, protectedRepositories, now)
	if err != nil {
		return verifiedPolicyReadToken{}, verificationFailure(result, err)
	}
	paths, ok := result.ReadableFilePaths()
	if !ok {
		return verifiedPolicyReadToken{}, verificationFailure(result, errors.New("single_file_paths are unreadable"))
	}
	// バイト列で比べる理由は Lambda Design の「GitHub Client」に記述する
	if !slices.Equal(paths, []string{repositorypolicy.Path}) {
		return verifiedPolicyReadToken{}, verificationFailure(result, errors.New("single_file_paths are not only the repository policy path"))
	}
	targets := make([]policyReadTarget, len(granted))
	for j, fullName := range granted {
		targets[j] = policyReadTarget{fullName: fullName, repositoryID: verified.grantedRepositoryIDs[j]}
	}
	return verifiedPolicyReadToken{token: token, targets: targets}, nil
}

// grantTerms は、照合の相手とする許可（発行許可かポリシー読み取り許可）の対象と Permissions である。
// name は、満たさなかった条件の文言で許可を指す呼び名である。
type grantTerms struct {
	name         string
	repositories []repository.FullName
	permissions  permission.Permissions
}

// verifiedTerms は、トークンを除く照合を通った発行結果の項目である。
type verifiedTerms struct {
	expiresAt    time.Time
	permissions  permission.Permissions
	repositories []github.EffectiveRepository
	// grantedRepositoryIDs は、許可の対象の順に、各対象に対応する実効対象の Repository ID を並べたものである
	grantedRepositoryIDs []githubid.RepositoryID
}

// verifyTerms は、INV-012 と INV-019 に共通する、トークンを除く条件を確かめる。
// 満たさなかった条件だけを返し、ErrVerificationFailure で包むのは呼び出し側である。
func verifyTerms(result github.IssuanceResult, terms grantTerms, allowedOwner authz.AllowedOwner, protectedRepositories protected.Repositories, now time.Time) (verifiedTerms, error) {
	expiresAt, ok := result.ExpiresAt()
	if !ok {
		return verifiedTerms{}, errors.New("expires_at is unreadable")
	}
	if !expiresAt.After(now) {
		return verifiedTerms{}, errors.New("expires_at is not after now")
	}
	repositories, ok := result.EffectiveRepositories()
	if !ok {
		return verifiedTerms{}, errors.New("repositories are unreadable")
	}
	grantedIDs, err := verifyRepositories(repositories, terms, allowedOwner.ID(), protectedRepositories)
	if err != nil {
		return verifiedTerms{}, err
	}
	permissions, ok := result.EffectivePermissions()
	if !ok {
		return verifiedTerms{}, errors.New("permissions are unreadable")
	}
	if !permissions.EqualsExceptImplied(terms.permissions) {
		return verifiedTerms{}, fmt.Errorf("permissions do not equal %s permissions except implied permissions", terms.name)
	}
	selection, ok := result.RepositorySelection()
	if !ok {
		return verifiedTerms{}, errors.New("repository_selection is unreadable")
	}
	if selection != github.RepositorySelectionSelected {
		return verifiedTerms{}, errors.New("repository_selection is not selected")
	}
	return verifiedTerms{expiresAt: expiresAt, permissions: permissions, repositories: repositories, grantedRepositoryIDs: grantedIDs}, nil
}

func verificationFailure(result github.IssuanceResult, condition error) error {
	return fmt.Errorf("%w: %w, github request id %q", ErrVerificationFailure, condition, result.GitHubRequestID())
}

// verifyRepositories は、実効対象と許可の対象が完全名で一対一に対応し、各実効対象が許可オーナーの有効なリポジトリで、保護対象と名前と ID が食い違わないかを確かめ、
// 許可の対象の順に、対応する実効対象の Repository ID を返す。
// 件数が一致し、どの実効対象も未対応の対象のちょうど一つに対応すれば、余分・欠落・重複はない。
// 発行結果は照合の前の値で ID を int64 のまま持つため、Repository ID はここで ID の形式を確かめる。
// 所有オーナーの Owner ID は、ゼロ値でない許可オーナーの Owner ID と一致すれば ID の形式も満たすため、一致だけを比べる。
func verifyRepositories(repositories []github.EffectiveRepository, terms grantTerms, allowedOwnerID githubid.OwnerID, protectedRepositories protected.Repositories) ([]githubid.RepositoryID, error) {
	granted := terms.repositories
	// 件数を先に比べ、名前の総当たりを許可の対象の件数の上限の二乗に収める
	if len(repositories) != len(granted) {
		return nil, fmt.Errorf("repositories count does not match %s", terms.name)
	}
	grantedIDs := make([]githubid.RepositoryID, len(granted))
	matched := make([]bool, len(granted))
	seenIDs := make(map[githubid.RepositoryID]bool, len(repositories))
	for i, effective := range repositories {
		id, err := githubid.NewRepositoryID(effective.RepositoryID())
		if err != nil {
			return nil, fmt.Errorf("repositories[%d] id is not a positive integer", i)
		}
		if seenIDs[id] {
			return nil, fmt.Errorf("repositories[%d] id is a duplicate", i)
		}
		seenIDs[id] = true
		if effective.OwnerID() != allowedOwnerID.Int64() {
			return nil, fmt.Errorf("repositories[%d] owner.id does not match allowed owner id", i)
		}
		j := unmatchedGrantIndex(granted, matched, effective.FullName())
		if j < 0 {
			return nil, fmt.Errorf("repositories[%d] full_name does not match %s", i, terms.name)
		}
		matched[j] = true
		grantedIDs[j] = id
		if err := protectedRepositories.Verify(id, effective.FullName()); err != nil {
			return nil, fmt.Errorf("repositories[%d]: %w", i, err)
		}
	}
	return grantedIDs, nil
}

// unmatchedGrantIndex は、まだ実効対象と対応していない許可の対象のうち、完全名が一致するものの位置を返す。なければ -1 を返す。
// FullName は map のキーに使えないため、比較規則を repository.FullName.Equal に閉じたまま総当たりで探す。
func unmatchedGrantIndex(granted []repository.FullName, matched []bool, fullName repository.FullName) int {
	for j, name := range granted {
		if !matched[j] && name.Equal(fullName) {
			return j
		}
	}
	return -1
}
