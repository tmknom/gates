package authn

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
)

// keyRetrievalInterval の値の理由は Lambda Design の JWKS の再取得の上限に置く。
const keyRetrievalInterval = time.Minute

// issuerKeySet は Issuer の公開鍵を取得して保持する oidc.KeySet である。
// go-oidc の RemoteKeySet を使わないのは、次の点で要件に合わないため。
// 取得の失敗を分類できる形で返さず、応答の大きさに上限がなく、応答本文をエラーに含め、kid のない JWT を保持した鍵のすべてで試行し、再取得の頻度を制限しない。
type issuerKeySet struct {
	client *http.Client
	issuer *url.URL
	now    func() time.Time

	// mu を取得の通信中も保持する。同時の呼び出しで取得が重複せず、待っていた呼び出しが前の呼び出しの取得結果で検証するため。
	// 待つ側は ctx の中断に反応しないが、Lambda は同時に一つの要求しか処理せず、待ちが起きない。
	mu      sync.Mutex
	jwksURL *url.URL
	keys    jose.JSONWebKeySet
	// retrievalStartedAt は前回の取得の開始時刻で、ゼロ値は取得を一度も試みていないことを示す。
	retrievalStartedAt time.Time
	// retrievalFailure は前回の取得の失敗の種別で、成功していれば nil である。
	retrievalFailure error
}

// Option は Discovery と JWKS の取得の設定を変える。
type Option func(*issuerKeySet)

// WithTransport は取得に使う Transport を差し替える。テストが httptest の TLS サーバーの証明書を信頼するために使う。
func WithTransport(transport http.RoundTripper) Option {
	return func(keySet *issuerKeySet) { keySet.client.Transport = transport }
}

func newIssuerKeySet(issuer *url.URL, now func() time.Time, opts ...Option) *issuerKeySet {
	keySet := &issuerKeySet{
		client: &http.Client{
			// 固定した Issuer の外から文書を読まないよう、リダイレクトに従わず 200 以外として分類する
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		issuer: issuer,
		now:    now,
	}
	for _, opt := range opts {
		opt(keySet)
	}
	return keySet
}

// VerifySignature は、kid のある JWT を保持している公開鍵で検証できなければ、再取得の上限の範囲で取得し直して、もう一度だけ検証する。
// 取得の失敗は失敗の種別を包んで返し、それ以外の拒否は種別を包まずに返す。
func (k *issuerKeySet) VerifySignature(ctx context.Context, jwt string) ([]byte, error) {
	// go-oidc の解析は JSON 形式と空白を含む JWT を通すため、受理する形をここで限る
	signature, err := jose.ParseSignedCompact(jwt, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		return nil, fmt.Errorf("parse signature: %w", err)
	}
	if err := checkJOSEHeader(jwt); err != nil {
		return nil, err
	}
	// 取得しても結果が変わらない
	if signature.Signatures[0].Header.KeyID == "" {
		return nil, errors.New("jwt has no key id")
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	// 検証できる間は取得し直さず、Issuer が取り下げた鍵も受理し続ける。Security Design が受容したリスクである
	payload, err := k.verify(signature)
	if err == nil {
		return payload, nil
	}

	// 単調時計の読みを外し、時間条件と同じ壁時計で比べる。単調時計は実行環境の凍結中に進まない可能性があり、上限が実時間で延びるため
	now := k.now().Round(0)
	// retrievalStartedAt がゼロ値なら経過時間は time.Duration の上限に飽和し、上限に当たらない。
	// 壁時計が戻ると経過時間が負になる。上限に当てると、戻った分だけ取得が止まる
	if elapsed := now.Sub(k.retrievalStartedAt); elapsed >= 0 && elapsed < keyRetrievalInterval {
		if k.retrievalFailure != nil {
			return nil, fmt.Errorf("%w: key retrieval is limited after the previous retrieval failed", k.retrievalFailure)
		}
		return nil, err
	}
	k.retrievalStartedAt = now
	k.retrievalFailure = nil
	if err := k.retrieve(ctx); err != nil {
		k.retrievalFailure = retrievalFailureKind(err)
		return nil, err
	}
	return k.verify(signature)
}

// retrievalFailureKind は取得の失敗から種別だけを取り出す。応答のステータスのような内容を、後の呼び出しの失敗に持ち越さないため。
// retrieve の失敗はどれも ErrTransientFailure か ErrInternalFailure を包む。
func retrievalFailureKind(err error) error {
	if errors.Is(err, ErrTransientFailure) {
		return ErrTransientFailure
	}
	return ErrInternalFailure
}

// verify は mu を保持して呼ぶ。
// go-jose は同じ kid の先頭の鍵を使うため、一意に選べない JWT を先に拒否する。
// JWKS の取得は同じ kid の鍵を拒否しない。選択できるかは JWT ごとに決まり、ほかの kid の JWT の検証を止めないため。
func (k *issuerKeySet) verify(signature *jose.JSONWebSignature) ([]byte, error) {
	// 取得が使えない鍵を除いてあるため、件数は使える鍵の数である
	if len(k.keys.Key(signature.Signatures[0].Header.KeyID)) > 1 {
		return nil, errors.New("key id does not identify a single key")
	}
	return signature.Verify(&k.keys)
}

// retrieve は mu を保持して呼ぶ。
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
