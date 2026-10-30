package authn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/request"
)

// clockSkew は iat の判定に使う許容時計差である。go-oidc の nbf の猶予は固定値で、テストがこの値との一致を確かめる。
const clockSkew = 5 * time.Minute

// Verifier は、固定設定と公開鍵の集合に照らして GATES 用 OIDC JWT を認証する。
type Verifier struct {
	issuer string
	keys   oidc.KeySet
	config *oidc.Config
}

// NewVerifier は固定設定を確かめ、Issuer の Discovery と JWKS から公開鍵を取得する Verifier を作る。
// 通信は最初の Authenticate まで行わない。
func NewVerifier(issuer, audience string, now func() time.Time, opts ...Option) (*Verifier, error) {
	issuerURL, err := parseIssuer(issuer)
	if err != nil {
		return nil, err
	}
	if audience == "" {
		return nil, fmt.Errorf("%w: must not be empty", ErrInvalidAudience)
	}
	return newVerifier(issuer, audience, newIssuerKeySet(issuerURL, opts...), now), nil
}

// parseIssuer は Domain Model の認証の固定設定の条件を Issuer について確かめる。
// go-oidc は JWT の iss を設定の文字列と比べ、Discovery の照合は解析後の値と比べる。
// 解析で変わる値を通すと、Discovery の照合を通ったまま、設定の誤りがすべての JWT の不正に見える。
// Host から組み立て直して比べるのは、ほかの要素を列挙すると、列挙から漏れた表現 (空の Query など) を通すため。
// 空のポートとホスト名のない値は組み立て直しても一致するため、別に判定する。
func parseIssuer(issuer string) (*url.URL, error) {
	issuerURL, err := url.Parse(issuer)
	if err != nil || issuerURL.Hostname() == "" || strings.HasSuffix(issuerURL.Host, ":") ||
		(&url.URL{Scheme: "https", Host: issuerURL.Host}).String() != issuer {
		return nil, fmt.Errorf("%w: must be an https URL with only a host and an optional port", ErrInvalidIssuer)
	}
	return issuerURL, nil
}

// newVerifier は now を go-oidc の設定に渡し、exp、nbf と Authenticate の iat を同じ時計で判定する。
func newVerifier(issuer, audience string, keys oidc.KeySet, now func() time.Time) *Verifier {
	config := &oidc.Config{
		ClientID:             audience,
		SupportedSigningAlgs: []string{oidc.RS256},
		Now:                  now,
	}
	return &Verifier{issuer: issuer, keys: keys, config: config}
}

// retrievalRecorder は公開鍵の取得の失敗を Verify の戻り値の外に持ち出す。
// go-oidc の Verify は oidc.KeySet の error を %v で包むため、戻り値からは errors.Is で判定できない。
type retrievalRecorder struct {
	keys    oidc.KeySet
	failure error
}

func (r *retrievalRecorder) VerifySignature(ctx context.Context, jwt string) ([]byte, error) {
	payload, err := r.keys.VerifySignature(ctx, jwt)
	if errors.Is(err, ErrTransientFailure) || errors.Is(err, ErrInternalFailure) {
		r.failure = err
	}
	return payload, err
}

// Authenticate は、未検証の GATES 用 OIDC JWT から Caller を確定する。
func (v *Verifier) Authenticate(ctx context.Context, rawIDToken request.RawIDToken) (authz.Caller, error) {
	// 記録先を呼び出しごとに分けるため、呼び出しごとに組み立てる。go-oidc の検証器は値を持つだけで、組み立ては軽い
	recorder := &retrievalRecorder{keys: v.keys}
	idToken, err := oidc.NewVerifier(v.issuer, recorder, v.config).Verify(ctx, rawIDToken.Expose())
	if err != nil {
		if recorder.failure != nil {
			return authz.Caller{}, recorder.failure
		}
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
	if idToken.IssuedAt.After(v.config.Now().Add(clockSkew)) {
		return authz.Caller{}, fmt.Errorf("%w: iat must not be later than the current time plus the clock skew", ErrAuthenticationFailure)
	}

	// Claims は encoding/json (v1) で復号するため、重複したキーを拒否する callerFromClaims に生の JSON を渡す。
	// Verify が同じ Payload を復号済みで失敗しない。失敗しても raw が空のまま callerFromClaims が拒否する
	var raw json.RawMessage
	_ = idToken.Claims(&raw)
	return callerFromClaims(raw)
}
