package oidc

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/credential"
)

// Retrieve は、audience の OIDC JWT を取得する。
func (c *Client) Retrieve(ctx context.Context, audience string) (credential.OIDCJWT, error) {
	ctx, cancel := context.WithTimeout(ctx, retrievalTimeout)
	defer cancel()

	response, err := c.httpClient.Do(c.newRequest(ctx, audience))
	if err != nil {
		return credential.OIDCJWT{}, communicationError{cause: err}
	}
	defer func() { _ = response.Body.Close() }() // 読み終えた本文の Close の失敗は結果に影響しない

	body, err := readBody(response)
	if err != nil {
		return credential.OIDCJWT{}, err
	}
	if response.StatusCode != http.StatusOK {
		return credential.OIDCJWT{}, statusFailure(response.StatusCode, body)
	}
	return parseIDTokenValue(body)
}

// newRequest は、audience を既存のクエリの後ろに足した GET 要求を作る。
// ACTIONS_ID_TOKEN_REQUEST_URL は api-version のクエリを既に持つため、置き換えずに足す。
func (c *Client) newRequest(ctx context.Context, audience string) *http.Request {
	target := *c.requestURL
	parameter := "audience=" + url.QueryEscape(audience)
	if target.RawQuery == "" {
		target.RawQuery = parameter
	} else {
		target.RawQuery += "&" + parameter
	}
	// http.NewRequestWithContext は解析済みの URL を解析し直す。失敗しない分岐を増やすだけのため使わない
	request := (&http.Request{
		Method: http.MethodGet,
		URL:    &target,
		Header: http.Header{},
	}).WithContext(ctx)
	request.Header.Set("Authorization", "Bearer "+c.requestToken.Expose())
	request.Header.Set("Accept", "application/json")
	return request
}

// readBody は、上限までの本文を読む。上限を超えたら、本文を含めずに error を返す。
func readBody(response *http.Response) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(response.Body, responseSizeLimit+1))
	if err != nil {
		return nil, communicationError{cause: err}
	}
	if len(body) > responseSizeLimit {
		return nil, fmt.Errorf("the response with HTTP status %d exceeds %d bytes", response.StatusCode, responseSizeLimit)
	}
	return body, nil
}

// statusFailure は、200 以外のステータスの失敗を作る。
// 4xx と 5xx は原因の手がかりとして本文の先頭を含め、それ以外は本文を含めない。
func statusFailure(status int, body []byte) error {
	if status < http.StatusBadRequest {
		return fmt.Errorf("HTTP status %d", status)
	}
	return fmt.Errorf("HTTP status %d: %s", status, body[:min(len(body), errorBodyLimit)])
}

// parseIDTokenValue は、200 の応答本文から OIDC JWT を取り出す。
// 1 行の値だけを受け付ける理由は、GitHub Actions Design の「OIDC JWT の取得」に記述する。
func parseIDTokenValue(body []byte) (credential.OIDCJWT, error) {
	var parsed struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil || parsed.Value == "" || strings.ContainsAny(parsed.Value, "\r\n") {
		return credential.OIDCJWT{}, errors.New("the response with HTTP status 200 has no single-line value")
	}
	return credential.NewOIDCJWT(parsed.Value), nil
}
