package oidc

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/credential"
)

// 値の理由は GitHub Actions Design の「OIDC JWT の取得」に記述する。
const (
	connectTimeout    = 10 * time.Second
	retrievalTimeout  = 30 * time.Second
	responseSizeLimit = 64 * 1024
	errorBodyLimit    = 4096
)

// Client は、GitHub OIDC Provider から OIDC JWT を取得する。
type Client struct {
	requestURL   *url.URL
	requestToken credential.OIDCRequestToken
	httpClient   *http.Client
}

// Option は Client の設定を変える。
type Option func(*Client)

// WithTransport は要求に使う Transport を差し替える。テストが httptest の TLS サーバーの証明書を信頼するために使う。
// http.Client ごとではなく Transport だけを差し替えるのは、リダイレクトに従わない設定を保つため。
func WithTransport(transport http.RoundTripper) Option {
	return func(c *Client) { c.httpClient.Transport = transport }
}

// NewClient は、requestURL の OIDC JWT を要求用トークンで取得する Client を作る。
// requestURL が HTTPS でなければ、接続せずに error を返す。要求用トークンを平文で送らないため、HTTP を許す Option は持たない。
func NewClient(requestURL string, requestToken credential.OIDCRequestToken, opts ...Option) (*Client, error) {
	parsed, err := url.Parse(requestURL)
	if err != nil {
		return nil, errors.New("the OIDC request URL is not a valid URL")
	}
	if parsed.Scheme != "https" || parsed.Host == "" {
		return nil, errors.New("the OIDC request URL must be an https URL with a host")
	}

	// DefaultTransport の複製に、接続の期限だけを足す
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: connectTimeout}).DialContext
	c := &Client{
		requestURL:   parsed,
		requestToken: requestToken,
		httpClient: &http.Client{
			Transport: transport,
			// 3xx を成功として扱わず、応答のステータスで失敗にする
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}
