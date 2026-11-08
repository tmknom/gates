// Package handler は Lambda Handler で、一回の発行処理を Processing Flow の順に進め、監査記録を出力して応答を返す。
// 手順は Lambda Design の「Processing Flow」に、応答の形は「Responses」に、監査記録は「Audit Record」に従う。
package handler

import (
	"context"
	"log/slog"
	"runtime/debug"

	"github.com/tmknom/gates/internal/audit"
	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/caller"
	"github.com/tmknom/gates/internal/errorcode"
	"github.com/tmknom/gates/internal/nonzero"
	"github.com/tmknom/gates/internal/request"
	"github.com/tmknom/gates/internal/tokenissuer"
)

// oidcVerifier は、OIDC Verifier のうち Lambda Handler が使う操作である。
type oidcVerifier interface {
	Authenticate(ctx context.Context, unverifiedIDToken request.UnverifiedIDToken) (caller.Caller, error)
}

// installationTokenIssuer は、Token Issuer のうち Lambda Handler が使う操作である。
type installationTokenIssuer interface {
	IssueInstallationToken(ctx context.Context, grant tokenissuer.GrantFunc) (tokenissuer.InstallationToken, error)
}

// Handler は Lambda Handler である。
type Handler struct {
	verifier    oidcVerifier
	tokenIssuer installationTokenIssuer
	owner       authz.AllowedOwner
	log         *slog.Logger
}

// NewHandler は、Lambda Handler を作る。
// log には、監査記録と回復した panic のログを書く。
func NewHandler(verifier oidcVerifier, tokenIssuer installationTokenIssuer, owner authz.AllowedOwner, log *slog.Logger) *Handler {
	// ゼロ値の許可オーナーは、事前認可と Grant の照合先にならない
	nonzero.Require(owner)
	return &Handler{verifier: verifier, tokenIssuer: tokenIssuer, owner: owner, log: log}
}

// Handle は、一回の発行処理を行い、応答の JSON を返す。
// error を返さず、処理中の panic も internal_error の応答にする。応答を書けた Invoke を FunctionError にしないためである。
func (h *Handler) Handle(ctx context.Context, requestID RequestID, payload []byte) []byte {
	record := audit.NewRecord(requestID.String())
	result, resp := h.process(ctx, requestID, payload, &record)
	// 監査記録は発行処理 1 回につき 1 行で、応答を返す直前のここだけで書く
	record.Write(h.log, result)
	return resp
}

// process は、発行処理の結果と応答を作り、処理中の panic を回復する。
// 監査記録の出力を回復の外に置き、panic で終わった発行処理も、ほかの結果と同じ 1 か所で監査記録を書く。
func (h *Handler) process(ctx context.Context, requestID RequestID, payload []byte, record *audit.Record) (result audit.Result, resp response) {
	defer func() {
		// panic の値は捨てる。資格情報が入っていない保証がないため、応答、監査記録、ログのどれにも書かない
		if recover() == nil {
			return
		}
		h.log.Error("panic recovered", slog.String("request_id", requestID.String()), slog.String("stack", string(debug.Stack())))
		result = audit.Failed(errorcode.InternalError, reasonPanicRecovered)
		resp = encodeFailure(requestID.String(), errorcode.InternalError)
		// 回復の外のため、符号化できない応答は FunctionError になる。Handler が応答を書けない場合の扱いと同じである
		nonzero.Require(resp)
	}()
	installationToken, err := h.runProcessingFlow(ctx, payload, record)
	if err != nil {
		code := errorCodeOf(err)
		result, resp = failedResult(code, err), encodeFailure(requestID.String(), code)
	} else {
		result, resp = audit.Succeeded(), encodeSuccess(requestID.String(), installationToken)
	}
	// 相関 ID は RequestID の値で、成功応答のほかの値は GitHub Client が JSON として復号した値のため、符号化できない応答はプログラムの誤りである。
	// 回復の範囲で止め、internal_error の応答と監査記録を返す
	nonzero.Require(resp)
	return result, resp
}

// runProcessingFlow は、Processing Flow の手順を監査記録の出力の前まで順に行い、各手順で確定した値を監査記録に詰める。
func (h *Handler) runProcessingFlow(ctx context.Context, payload []byte, record *audit.Record) (tokenissuer.InstallationToken, error) {
	unverifiedIDToken, tokenRequest, err := request.Validate(payload)
	if err != nil {
		return tokenissuer.InstallationToken{}, err
	}
	record.SetTokenRequest(tokenRequest)
	verified, err := h.verifier.Authenticate(ctx, unverifiedIDToken)
	if err != nil {
		return tokenissuer.InstallationToken{}, err
	}
	record.SetCaller(verified)
	if err := authz.Authorize(verified, tokenRequest, h.owner); err != nil {
		return tokenissuer.InstallationToken{}, err
	}
	// 要求の ctx を別の期限で包まない。失効のタイムアウトは要求の ctx の期限から決まる（Lambda Design の「Call Time Budget」）
	return h.tokenIssuer.IssueInstallationToken(ctx, func(installation authz.Installation) (authz.TokenGrant, error) {
		// Grant が拒否しても、Installation の解決には成功しているため、判定の前に詰める
		record.SetInstallation(installation)
		return authz.Grant(verified, tokenRequest, h.owner, installation)
	})
}
