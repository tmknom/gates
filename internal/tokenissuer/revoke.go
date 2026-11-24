package tokenissuer

import (
	"context"
	"fmt"
	"time"

	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/revocation"
)

// responseReserve は、関数の残り時間のうち、応答を書くために失効に使わない時間である。値の理由は Lambda Design の「Call Time Budget」に記述する。
const responseReserve = time.Second

// revokeAfterVerificationFailure は、照合を通らなかった発行結果にトークン文字列があれば失効を試み、照合失敗に失効の結果を添える。
func (i *TokenIssuer) revokeAfterVerificationFailure(ctx context.Context, result github.IssuanceResult, verification error) error {
	token, ok := result.Token()
	if !ok {
		return &VerificationFailure{verification: verification, outcome: revocation.NotAttempted}
	}
	ctx, cancel := revocationContext(ctx)
	defer cancel()
	if err := i.client.Revoke(ctx, token); err != nil {
		return &VerificationFailure{verification: verification, outcome: revocation.Failed, revocationErr: err}
	}
	return &VerificationFailure{verification: verification, outcome: revocation.Succeeded}
}

// revokePolicyReadToken は、ポリシー読み取りトークンの失効を試み、その結果と、段階の error を返す。
// 失効を試みて失敗し、段階が error で終わる場合は、その文言に失効の失敗の理由を添える。
// 失効の失敗は失敗の種別ではないため、段階の error の Unwrap に含めず、文言だけを足す。
func (i *TokenIssuer) revokePolicyReadToken(ctx context.Context, token github.PolicyReadToken, stageErr error) (revocation.Outcome, error) {
	ctx, cancel := revocationContext(ctx)
	defer cancel()
	revocationErr := i.client.RevokePolicyReadToken(ctx, token)
	if revocationErr == nil {
		return revocation.Succeeded, stageErr
	}
	if stageErr != nil {
		stageErr = fmt.Errorf("%w; %s", stageErr, revocationErr.Error())
	}
	return revocation.Failed, stageErr
}

// revocationContext は、要求の ctx の中断と期限を引き継がず、要求の ctx の期限から responseReserve を除いた時刻で打ち切る ctx を作る。
// 要求の ctx の期限は関数の期限であり、失効のタイムアウトは GitHub Client が足す。
func revocationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	// context.WithoutCancel は期限も外すため、先に読む
	deadline, hasDeadline := ctx.Deadline()
	revocationCtx := context.WithoutCancel(ctx)
	if !hasDeadline {
		return revocationCtx, func() {}
	}
	return context.WithDeadline(revocationCtx, deadline.Add(-responseReserve))
}
