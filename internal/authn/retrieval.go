package authn

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// 値の理由は Lambda Design の取得の上限に置く。
const (
	keyRetrievalTimeout   = 5 * time.Second
	keyRetrievalSizeLimit = 64 * 1024
)

// retrieveDocument は Discovery か JWKS の本文を取得し、取得の失敗を分類する。
func retrieveDocument(ctx context.Context, client *http.Client, documentURL *url.URL) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, keyRetrievalTimeout)
	defer cancel()

	// http.NewRequestWithContext は URL の文字列を解析し直す。解析済みの URL では失敗しない分岐を増やすだけのため使わない
	request := (&http.Request{
		Method: http.MethodGet,
		URL:    documentURL,
		// JWKS の応答はキャッシュを許す（max-age=3600 を確認）ため、ローテーションの取得し直しで途中のキャッシュから古い鍵を受け取らないようにする
		Header: http.Header{"Cache-Control": {"no-cache"}},
	}).WithContext(ctx)
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: request failed", ErrTransientFailure)
	}
	// 読み取りの結果は本文の読み取りで確かめており、Close の失敗は結果に影響しない
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return nil, statusFailure(response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, keyRetrievalSizeLimit+1))
	if err != nil {
		return nil, fmt.Errorf("%w: response body cannot be read", ErrTransientFailure)
	}
	if len(body) > keyRetrievalSizeLimit {
		return nil, fmt.Errorf("%w: response exceeds %d bytes", ErrInternalFailure, keyRetrievalSizeLimit)
	}
	return body, nil
}

func statusFailure(status int) error {
	if status >= http.StatusInternalServerError || status == http.StatusForbidden || status == http.StatusTooManyRequests {
		return fmt.Errorf("%w: status %d", ErrTransientFailure, status)
	}
	return fmt.Errorf("%w: status %d", ErrInternalFailure, status)
}
