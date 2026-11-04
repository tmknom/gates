package github

import (
	"context"
	"net/http"
	"net/url"
)

// 値の理由は Lambda Design の「GitHub Client」に記述する。
const (
	apiVersion = "2026-03-10"
	userAgent  = "gates"
)

// Client は GitHub API の呼び出しを担う GitHub Client である。
type Client struct {
	// API Host は固定設定であり、実行時の入力から変えられないよう、テストだけが Option で差し替える
	apiHost    *url.URL
	httpClient *http.Client
}

// Option は GitHub Client の設定を変える。
type Option func(*Client)

// WithAPIHost は API Host を差し替える。テストが httptest のサーバーに向けるために使う。
func WithAPIHost(apiHost *url.URL) Option {
	return func(c *Client) { c.apiHost = apiHost }
}

// WithTransport は要求に使う Transport を差し替える。テストが httptest の TLS サーバーの証明書を信頼するために使う。
func WithTransport(transport http.RoundTripper) Option {
	return func(c *Client) { c.httpClient.Transport = transport }
}

// NewClient は GitHub.com の API に要求を送る GitHub Client を作る。
func NewClient(opts ...Option) *Client {
	c := &Client{
		apiHost: &url.URL{Scheme: "https", Host: "api.github.com"},
		httpClient: &http.Client{
			// リダイレクトに従わず成功以外の応答として分類する。理由は Lambda Design の「GitHub Client」に記述する
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// newRequest は、credential を Bearer で送る要求を作る。credential は、要求ごとに GitHub App JWT か Installation Access Token のどちらかである。
// path は / で始める。JoinPath は、API Host のパスが空のとき / で始まらないパスを作るため使わない。
func (c *Client) newRequest(ctx context.Context, method, path string, credential redactedValue) *http.Request {
	// http.NewRequestWithContext は URL の文字列を解析し直す。解析済みの URL では失敗しない分岐を増やすだけのため使わない
	request := (&http.Request{
		Method: method,
		URL:    c.apiHost.ResolveReference(&url.URL{Path: path}),
		Header: http.Header{},
	}).WithContext(ctx)
	request.Header.Set("Authorization", "Bearer "+credential.expose())
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", apiVersion)
	request.Header.Set("User-Agent", userAgent)
	return request
}
