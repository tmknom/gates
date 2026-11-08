package tokenissuer

import (
	"context"
	"time"

	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/revocation"
)

// responseReserve は、関数の残り時間のうち、照合失敗の応答を書くために失効に使わない時間である。値の理由は Lambda Design の「Call Time Budget」に記述する。
const responseReserve = time.Second

// VerificationFailure は、照合失敗のエラーで、ErrVerificationFailure を満たし、失効の結果を持つ。
type VerificationFailure struct {
	// verification は、ErrVerificationFailure を包んだ照合の失敗である
	verification error
	outcome      revocation.Outcome
	// revocationErr は、失効を試みて失敗したときの理由である。
	// 失効の失敗は失敗の種別ではないため Unwrap に含めず、照合失敗をほかの種別として判定させない
	revocationErr error
}

// Error は、照合の失敗の文言に、失効を試みて失敗した場合だけその理由を添える。
func (e *VerificationFailure) Error() string {
	if e.outcome == revocation.Failed {
		return e.verification.Error() + "; " + e.revocationErr.Error()
	}
	return e.verification.Error()
}

// Unwrap は、照合の失敗を返す。
func (e *VerificationFailure) Unwrap() error {
	return e.verification
}

// RevocationOutcome は、失効の結果を返す。
func (e *VerificationFailure) RevocationOutcome() revocation.Outcome {
	return e.outcome
}

// revokeUnverified は、照合を通らなかった発行結果にトークン文字列があれば失効を試み、照合失敗に失効の結果を添える。
func (i *TokenIssuer) revokeUnverified(ctx context.Context, result github.IssuanceResult, verification error) error {
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
