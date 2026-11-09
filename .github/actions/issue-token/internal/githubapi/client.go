// Package githubapi は、GitHub API への失効の要求を送る。
package githubapi

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/communication"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/credential"
)

// 値の理由は GitHub Actions Design の「Revocation」に記述する。
const (
	apiVersion = "2026-03-10"
	userAgent  = "gates-issue-token"
)

// revocationTimeout の値の理由は、GitHub Actions Design の Retry and Timeout にある。
const revocationTimeout = 5 * time.Second

// Client は、GitHub API に失効の要求を送る。
type Client struct {
	// API Host は固定設定であり、実行時の入力から変えられないよう、テストだけが Option で差し替える
	apiHost    *url.URL
	httpClient *http.Client
}

// Option は Client の設定を変える。
type Option func(*Client)

// WithAPIHost は API Host を差し替える。テストが httptest のサーバーに向けるために使う。
func WithAPIHost(apiHost *url.URL) Option {
	return func(c *Client) { c.apiHost = apiHost }
}

// WithTransport は要求に使う Transport を差し替える。テストが httptest の TLS サーバーの証明書を信頼するために使う。
// http.Client ごとではなく Transport だけを差し替えるのは、リダイレクトに従わない設定を保つため。
func WithTransport(transport http.RoundTripper) Option {
	return func(c *Client) { c.httpClient.Transport = transport }
}

// NewClient は、GitHub.com の API に要求を送る Client を作る。
func NewClient(opts ...Option) *Client {
	c := &Client{
		apiHost: &url.URL{Scheme: "https", Host: "api.github.com"},
		httpClient: &http.Client{
			// 3xx を成功として扱わず、応答のステータスを呼び出し側に返す
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Revoke は、token をそのトークン自身で認証して失効の要求を送り、応答の HTTP ステータスを返す。
// 204 以外でも error にしない。失効はベストエフォートで、ステータスは処理を分ける失敗の種類ではなく、呼び出し側がそのままログに出す値であるため。
// 応答を得られなければ通信の失敗の error を返し、status は 0 になる。
// 成否はステータスだけで決まるため、応答本文は読まない。
func (c *Client) Revoke(ctx context.Context, token credential.InstallationAccessToken) (status int, err error) {
	ctx, cancel := context.WithTimeout(ctx, revocationTimeout)
	defer cancel()

	// http.NewRequestWithContext は URL の文字列を解析し直す。解析済みの URL では失敗しない分岐を増やすだけのため使わない
	request := (&http.Request{
		Method: http.MethodDelete,
		URL:    c.apiHost.ResolveReference(&url.URL{Path: "/installation/token"}),
		Header: http.Header{},
	}).WithContext(ctx)
	request.Header.Set("Authorization", "Bearer "+token.Expose())
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", apiVersion)
	request.Header.Set("User-Agent", userAgent)

	response, err := c.httpClient.Do(request)
	if err != nil {
		// url.Error の文言は URL を含むため、文言は communication.Failure に任せる
		return 0, communication.Failed(err)
	}
	// 本文を読まず、Close の失敗は結果に影響しない
	defer func() { _ = response.Body.Close() }()
	return response.StatusCode, nil
}
