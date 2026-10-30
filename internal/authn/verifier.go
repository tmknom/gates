package authn

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/request"
)

// clockSkew は iat の判定に使う許容時計差である。go-oidc の nbf の猶予は固定値で、テストがこの値との一致を確かめる。
const clockSkew = 5 * time.Minute

// Verifier は、固定設定と公開鍵の集合に照らして GATES 用 OIDC JWT を認証する。
type Verifier struct {
	idTokenVerifier *oidc.IDTokenVerifier
	now             func() time.Time
}

// NewVerifier は now を go-oidc にも渡し、exp、nbf、iat を同じ時計で判定する。
func NewVerifier(issuer, audience string, keys oidc.KeySet, now func() time.Time) *Verifier {
	config := &oidc.Config{
		ClientID:             audience,
		SupportedSigningAlgs: []string{oidc.RS256},
		Now:                  now,
	}
	return &Verifier{idTokenVerifier: oidc.NewVerifier(issuer, keys, config), now: now}
}

// Authenticate は、未検証の GATES 用 OIDC JWT から Caller を確定する。拒否はすべて ErrAuthenticationFailure を包む。
func (v *Verifier) Authenticate(ctx context.Context, rawIDToken request.RawIDToken) (authz.Caller, error) {
	idToken, err := v.idTokenVerifier.Verify(ctx, rawIDToken.Expose())
	if err != nil {
		// ErrAuthenticationFailure の文言の規則に従い、error を包まない
		return authz.Caller{}, fmt.Errorf("%w: signature, issuer, audience, or time condition is not satisfied", ErrAuthenticationFailure)
	}

	// go-oidc は aud の含有だけを確かめるため、ここでは要素数だけを判定する
	if len(idToken.Audience) != 1 {
		return authz.Caller{}, fmt.Errorf("%w: aud must have exactly one value", ErrAuthenticationFailure)
	}
	// go-oidc は iat を読むだけで検証しない
	if idToken.IssuedAt.IsZero() {
		return authz.Caller{}, fmt.Errorf("%w: iat is required", ErrAuthenticationFailure)
	}
	if idToken.IssuedAt.After(v.now().Add(clockSkew)) {
		return authz.Caller{}, fmt.Errorf("%w: iat must not be later than the current time plus the clock skew", ErrAuthenticationFailure)
	}

	// Claims は encoding/json (v1) で復号するため、重複したキーを拒否する callerFromClaims に生の JSON を渡す。
	// Verify が同じ Payload を復号済みで失敗しない。失敗しても raw が空のまま callerFromClaims が拒否する
	var raw json.RawMessage
	_ = idToken.Claims(&raw)
	return callerFromClaims(raw)
}
