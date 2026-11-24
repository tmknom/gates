package tokenissuer

import (
	"context"
	"fmt"
	"time"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/repositorypolicy"
	"github.com/tmknom/gates/internal/revocation"
)

// repositoryPolicyFetchBudget は、リポジトリポリシーの取得の合計の予算で、最初の取得から数える。値の理由は Lambda Design の「Call Time Budget」に記述する。
const repositoryPolicyFetchBudget = 15 * time.Second

// repositoryPolicyReading は、リポジトリポリシーを読んだ対象と、その取得の結果の組である。
type repositoryPolicyReading struct {
	target      policyReadTarget
	fetchResult repositorypolicy.FetchResult
}

// readRepositoryPolicies は、ポリシー読み取りトークンの発行、照合、各対象のリポジトリポリシーの取得、失効を行い、
// ポリシー読み取り許可の対象の順に、各対象の取得の結果と照合済みの Repository ID を返す。
// ポリシー読み取りトークンは戻り値にもエラーにも出さない。
// outcome はポリシー読み取りトークンの失効の結果で、error を返すときも返す。トークン文字列を読めなかった場合は試みなかったことを返す。
func (i *TokenIssuer) readRepositoryPolicies(ctx context.Context, appJWT github.AppJWT, grant authz.PolicyReadGrant) (readings []repositoryPolicyReading, outcome revocation.Outcome, err error) {
	result, err := i.client.IssuePolicyReadToken(ctx, appJWT, grant)
	if err != nil {
		// 発行 API の失敗は発行結果を持たず、トークン文字列がないため失効を試みない
		return nil, revocation.NotAttempted, fmt.Errorf("issue policy read token: %w", err)
	}
	outcome = revocation.NotAttempted
	if token, ok := result.PolicyReadToken(); ok {
		// 照合、取得、panic のどれで抜けても失効を試みるよう、トークン文字列を読んだ直後に置く（Lambda Design の「GitHub App JWT」）
		defer func() { outcome, err = i.revokePolicyReadToken(ctx, token, err) }()
	}
	verified, err := verifyPolicyReadToken(result, grant, i.allowedOwner, i.protectedRepositories, i.now())
	if err != nil {
		// 段階名を足す理由は Lambda Design の「応答とエラー文言に出す情報」に記述する
		return nil, outcome, fmt.Errorf("issue policy read token: %w", err)
	}
	readings, err = i.fetchRepositoryPolicies(ctx, verified)
	return readings, outcome, err
}

// fetchRepositoryPolicies は、照合済みのポリシー読み取りトークンの対象を順に取得し、取得の失敗があれば残りを取得せずに返す。
// リポジトリポリシーがない、ファイルでない、大きすぎるという結果は失敗ではなく、残りの取得を続ける。理由は Lambda Design の「Processing Flow」に記述する。
func (i *TokenIssuer) fetchRepositoryPolicies(ctx context.Context, verified verifiedPolicyReadToken) ([]repositoryPolicyReading, error) {
	// 各取得のタイムアウトは GitHub Client が足し、この期限との短いほうになる
	ctx, cancel := context.WithTimeout(ctx, repositoryPolicyFetchBudget)
	defer cancel()

	readings := make([]repositoryPolicyReading, 0, len(verified.targets))
	for j, target := range verified.targets {
		fetchResult, err := i.client.FetchRepositoryPolicy(ctx, verified.token, target.fullName)
		if err != nil {
			return nil, fmt.Errorf("fetch repository policy: repositories[%d]: %w", j, err)
		}
		readings = append(readings, repositoryPolicyReading{target: target, fetchResult: fetchResult})
	}
	return readings, nil
}
