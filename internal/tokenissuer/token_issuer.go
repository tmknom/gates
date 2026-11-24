package tokenissuer

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/nonzero"
	"github.com/tmknom/gates/internal/protected"
	"github.com/tmknom/gates/internal/repositorypolicy"
	"github.com/tmknom/gates/internal/revocation"
)

// TokenIssuer は、KMS Signer と GitHub Client を使って Installation Access Token を発行する Token Issuer である。
// 入口を struct のメソッドにするのは、KMS Signer、GitHub Client、許可オーナー、保護対象の一覧、リポジトリポリシーの適用を要求をまたいで保持するためである。
type TokenIssuer struct {
	signer                kmsSigner
	client                *github.Client
	allowedOwner          authz.AllowedOwner
	protectedRepositories protected.Repositories
	enforcement           authz.RepositoryPolicyEnforcement
	now                   func() time.Time
}

// kmsSigner は、KMS Signer のうち Token Issuer が使う操作である。
// テストが AWS SDK に依存せずに署名を差し替えるため、interface で受ける。
type kmsSigner interface {
	Sign(ctx context.Context) (github.AppJWT, error)
}

// Authorization は、Authorization コンポーネントのうち Token Issuer が使う、取得した Installation からの許可の確定である。
// Token Issuer が Caller と発行要求を知らずに済むよう、Lambda Handler が要求ごとに渡す。
// 署名から照合までを一つの操作にまとめる理由は、Lambda Design の「GitHub App JWT」に記述する。
type Authorization interface {
	// GrantPolicyRead は、リポジトリポリシーを適用するデプロイで、ポリシー読み取り許可を確定する。
	GrantPolicyRead(installation authz.Installation) (authz.PolicyReadGrant, error)
	// GrantWithRepositoryPolicies は、リポジトリポリシーを適用するデプロイで、要求の対象の順に並べた取得の結果から発行許可を確定する。
	GrantWithRepositoryPolicies(installation authz.Installation, fetchResults []repositorypolicy.FetchResult) (authz.TokenGrant, error)
	// Grant は、リポジトリポリシーを適用しないデプロイで、発行許可を確定する。
	Grant(installation authz.Installation) (authz.TokenGrant, error)
}

// NewTokenIssuer は Token Issuer を作る。
func NewTokenIssuer(signer kmsSigner, client *github.Client, allowedOwner authz.AllowedOwner, protectedRepositories protected.Repositories, enforcement authz.RepositoryPolicyEnforcement, now func() time.Time) *TokenIssuer {
	// ゼロ値は Installation の取得先を決められず、照合でも Owner ID 0 の実効対象と一致し得るため作らない
	nonzero.Require(allowedOwner)
	// ゼロ値の一覧は照合で拒否されるが、どの発行結果も返せない Token Issuer を作らない
	nonzero.Require(protectedRepositories)
	// ゼロ値はどちらの手順にも当てられないため作らない（INV-018）
	nonzero.Require(enforcement)
	return &TokenIssuer{signer: signer, client: client, allowedOwner: allowedOwner, protectedRepositories: protectedRepositories, enforcement: enforcement, now: now}
}

// IssueInstallationAccessToken は、署名、Installation の解決、許可の確定、発行、発行結果の照合を一つの操作として行い、返却可能な発行結果だけを返す。
// リポジトリポリシーを適用するデプロイでは、発行許可の確定の前に、ポリシー読み取りトークンでリポジトリポリシーを読む。
// GitHub App JWT は 1 回だけ作り、Installation の解決と発行の両方に使い、戻り値にもエラーにも出さない。
// 失敗はどの段階でも失敗の種別のまま返す。照合失敗は *VerificationFailure で、利用者に返すトークンの失効の結果を持つ。
// 2 つ目の戻り値はポリシー読み取りトークンの失効の結果で、error を返すときも返す。ポリシー読み取りトークンの発行 API を呼ばなかった場合はゼロ値である。
func (i *TokenIssuer) IssueInstallationAccessToken(ctx context.Context, authorization Authorization) (ReturnableIssuanceResult, revocation.Outcome, error) {
	appJWT, err := i.signer.Sign(ctx)
	if err != nil {
		return ReturnableIssuanceResult{}, revocation.Outcome{}, err
	}
	installation, err := i.client.ResolveInstallation(ctx, appJWT, i.allowedOwner)
	if err != nil {
		// 段階名を足す理由は Lambda Design の「応答とエラー文言に出す情報」に記述する
		return ReturnableIssuanceResult{}, revocation.Outcome{}, fmt.Errorf("resolve installation: %w", err)
	}
	if i.enforcement == authz.RepositoryPolicyEnforced {
		return i.issueWithRepositoryPolicies(ctx, appJWT, installation, authorization)
	}
	returnable, err := i.issueWithoutRepositoryPolicies(ctx, appJWT, installation, authorization)
	return returnable, revocation.Outcome{}, err
}

// issueWithRepositoryPolicies は、ポリシー読み取り許可を確定し、リポジトリポリシーを読み、その取得の結果で発行許可を確定して発行し、
// 発行結果を発行許可とリポジトリポリシーを読んだ対象に照合する（Lambda Design の「Processing Flow」の手順 5 から 8）。
func (i *TokenIssuer) issueWithRepositoryPolicies(ctx context.Context, appJWT github.AppJWT, installation authz.Installation, authorization Authorization) (ReturnableIssuanceResult, revocation.Outcome, error) {
	policyReadGrant, err := authorization.GrantPolicyRead(installation)
	if err != nil {
		return ReturnableIssuanceResult{}, revocation.Outcome{}, err
	}
	readings, outcome, err := i.readRepositoryPolicies(ctx, appJWT, policyReadGrant)
	if errors.Is(err, ErrVerificationFailure) {
		// ポリシー読み取りトークンの照合失敗では利用者に返すトークンを発行しておらず、その失効を試みていない
		return ReturnableIssuanceResult{}, outcome, &VerificationFailure{verification: err, outcome: revocation.NotAttempted}
	}
	if err != nil {
		return ReturnableIssuanceResult{}, outcome, err
	}
	fetchResults := make([]repositorypolicy.FetchResult, len(readings))
	targets := make([]policyReadTarget, len(readings))
	for j, reading := range readings {
		fetchResults[j] = reading.fetchResult
		targets[j] = reading.target
	}
	tokenGrant, err := authorization.GrantWithRepositoryPolicies(installation, fetchResults)
	if err != nil {
		return ReturnableIssuanceResult{}, outcome, err
	}
	result, err := i.issue(ctx, appJWT, tokenGrant)
	if err != nil {
		return ReturnableIssuanceResult{}, outcome, err
	}
	returnable, err := verifyWithPolicyReadTargets(result, tokenGrant, i.allowedOwner, i.protectedRepositories, targets, i.now())
	if err != nil {
		return ReturnableIssuanceResult{}, outcome, i.revokeAfterVerificationFailure(ctx, result, err)
	}
	return returnable, outcome, nil
}

// issueWithoutRepositoryPolicies は、発行許可を確定して発行し、発行結果を発行許可に照合する（Lambda Design の「Processing Flow」の手順 7 と 8）。
func (i *TokenIssuer) issueWithoutRepositoryPolicies(ctx context.Context, appJWT github.AppJWT, installation authz.Installation, authorization Authorization) (ReturnableIssuanceResult, error) {
	tokenGrant, err := authorization.Grant(installation)
	if err != nil {
		return ReturnableIssuanceResult{}, err
	}
	result, err := i.issue(ctx, appJWT, tokenGrant)
	if err != nil {
		return ReturnableIssuanceResult{}, err
	}
	returnable, err := verify(result, tokenGrant, i.allowedOwner, i.protectedRepositories, i.now())
	if err != nil {
		return ReturnableIssuanceResult{}, i.revokeAfterVerificationFailure(ctx, result, err)
	}
	return returnable, nil
}

// issue は、発行許可で利用者に返すトークンを発行させる。
// 発行 API の失敗は発行結果を持たず、トークン文字列がないため失効を試みない。
func (i *TokenIssuer) issue(ctx context.Context, appJWT github.AppJWT, tokenGrant authz.TokenGrant) (github.IssuanceResult, error) {
	result, err := i.client.Issue(ctx, appJWT, tokenGrant)
	if err != nil {
		return github.IssuanceResult{}, fmt.Errorf("issue: %w", err)
	}
	return result, nil
}
