package github

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// 値の理由は Lambda Design の「Call Time Budget」に記述する。
const revocationTimeout = 5 * time.Second

// Revoke は、Installation Access Token を、そのトークン自身で認証して失効させる。204 だけを成功とし、再試行しない。
// 失効の失敗は失敗の種別ではないため（Domain Model の「Operations」）、どの sentinel error でも包まない。
// 成否はステータスだけで決まるため、応答本文は読まない。
func (c *Client) Revoke(ctx context.Context, token InstallationAccessToken) error {
	if token == (InstallationAccessToken{}) {
		// ゼロ値は発行応答から読み取ったトークンではなく、送る値がない
		panic("installation access token is not read from an issuance response")
	}
	ctx, cancel := context.WithTimeout(ctx, revocationTimeout)
	defer cancel()

	response, err := c.httpClient.Do(c.newRequest(ctx, http.MethodDelete, "/installation/token", token.redactedValue))
	if err != nil {
		// 要求の URL は固定のエンドポイントで、伏せる値を含まないため包む
		return fmt.Errorf("revocation request failed: %w", err)
	}
	// 本文を読まず、Close の失敗は結果に影響しない
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("revocation returned status %d, github request id %q", response.StatusCode, response.Header.Get("X-GitHub-Request-Id"))
	}
	return nil
}
