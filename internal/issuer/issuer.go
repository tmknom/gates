package issuer

import (
	"context"
	"fmt"
	"time"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/nonzero"
)

// kmsSigner は、KMS Signer のうち Token Issuer が使う操作である。
// テストが AWS SDK に依存せずに署名を差し替えるため、interface で受ける。
type kmsSigner interface {
	Sign(ctx context.Context) (github.AppJWT, error)
}

// GrantFunc は、取得した Installation から発行許可を確定する。
// Token Issuer が Caller と発行要求を知らずに済むよう、発行許可の確定を関数で受け取る。
// 署名から照合までを一つの操作にまとめる理由は、Lambda Design の「GitHub App JWT」に記述する。
type GrantFunc func(installation authz.Installation) (authz.TokenGrant, error)

// Issuer は、KMS Signer と GitHub Client を使って Installation Token を発行する Token Issuer である。
// 入口を struct のメソッドにするのは、KMS Signer、GitHub Client、許可オーナーを要求をまたいで保持するためである。
type Issuer struct {
	signer kmsSigner
	client *github.Client
	owner  authz.AllowedOwner
	now    func() time.Time
}

// NewIssuer は Token Issuer を作る。owner は固定設定の許可オーナーである。
func NewIssuer(signer kmsSigner, client *github.Client, owner authz.AllowedOwner, now func() time.Time) *Issuer {
	// ゼロ値は Installation の取得先を決められず、照合でも Owner ID 0 の実効対象と一致し得るため作らない
	nonzero.Require(owner)
	return &Issuer{signer: signer, client: client, owner: owner, now: now}
}

// IssueInstallationToken は、署名、Installation の解決、発行許可の確定、発行、発行結果の照合を一つの操作として行い、照合を通った発行結果だけを返す。
// GitHub App JWT は 1 回だけ作り、Installation の解決と発行の両方に使い、戻り値にもエラーにも出さない。
// 失敗はどの段階でも失敗の種別のまま返す。照合失敗は *VerificationFailure で、失効の結果を持つ。
func (i *Issuer) IssueInstallationToken(ctx context.Context, grant GrantFunc) (InstallationToken, error) {
	appJWT, err := i.signer.Sign(ctx)
	if err != nil {
		return InstallationToken{}, err
	}
	installation, err := i.client.ResolveInstallation(ctx, appJWT, i.owner)
	if err != nil {
		// 段階名を足す理由は Lambda Design の「応答とエラー文言に出す情報」に記述する
		return InstallationToken{}, fmt.Errorf("resolve installation: %w", err)
	}
	tokenGrant, err := grant(installation)
	if err != nil {
		return InstallationToken{}, err
	}
	// 発行 API の失敗は発行結果を持たず、トークン文字列がないため失効を試みない
	result, err := i.client.Issue(ctx, appJWT, tokenGrant)
	if err != nil {
		return InstallationToken{}, fmt.Errorf("issue: %w", err)
	}
	token, err := verify(result, tokenGrant, i.owner, i.now())
	if err != nil {
		return InstallationToken{}, i.revokeUnverified(ctx, result, err)
	}
	return token, nil
}
