package authn

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/go-jose/go-jose/v4"
)

// issuerKeySet は Issuer の公開鍵を取得して保持する oidc.KeySet である。
// 検証ライブラリの取得処理は失敗を分類できる形で返さないため、取得だけを GATES が担う。
type issuerKeySet struct {
	client *http.Client
	issuer *url.URL

	// mu を取得の通信中も保持する。同時の呼び出しで取得が重複せず、別の呼び出しの失敗を受け取らないため。
	// 待つ側は ctx の中断に反応しないが、Lambda は同時に一つの要求しか処理せず、待ちが起きない。
	mu      sync.Mutex
	jwksURL *url.URL
	keys    jose.JSONWebKeySet
}

// Option は Discovery と JWKS の取得の設定を変える。
type Option func(*issuerKeySet)

// WithTransport は取得に使う Transport を差し替える。テストが httptest の TLS サーバーの証明書を信頼するために使う。
func WithTransport(transport http.RoundTripper) Option {
	return func(keySet *issuerKeySet) { keySet.client.Transport = transport }
}

func newIssuerKeySet(issuer *url.URL, opts ...Option) *issuerKeySet {
	keySet := &issuerKeySet{
		client: &http.Client{
			// 固定した Issuer の外から文書を読まないよう、リダイレクトに従わず 200 以外として分類する
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		issuer: issuer,
	}
	for _, opt := range opts {
		opt(keySet)
	}
	return keySet
}

// VerifySignature は、kid のある JWT を保持している公開鍵で検証できなければ取得し直して、もう一度だけ検証する。
// 取得の失敗は失敗の種別を包んで返し、署名の不一致は種別を包まずに返す。
func (k *issuerKeySet) VerifySignature(ctx context.Context, jwt string) ([]byte, error) {
	// go-oidc の解析は JSON 形式と空白を含む JWT を通すため、受理する形をここで限る
	signature, err := jose.ParseSignedCompact(jwt, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		return nil, fmt.Errorf("parse signature: %w", err)
	}
	// 取得しても結果が変わらない
	if signature.Signatures[0].Header.KeyID == "" {
		return nil, errors.New("jwt has no key id")
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	if payload, err := signature.Verify(&k.keys); err == nil {
		return payload, nil
	}
	if err := k.retrieve(ctx); err != nil {
		return nil, err
	}
	return signature.Verify(&k.keys)
}

// retrieve は mu を保持して呼ぶ。失敗した取得の内容を保持しない。
func (k *issuerKeySet) retrieve(ctx context.Context) error {
	if k.jwksURL == nil {
		jwksURL, err := discoverJWKSURL(ctx, k.client, k.issuer)
		if err != nil {
			return err
		}
		k.jwksURL = jwksURL
	}
	keys, err := retrieveJWKS(ctx, k.client, k.jwksURL)
	if err != nil {
		return err
	}
	k.keys = keys
	return nil
}
