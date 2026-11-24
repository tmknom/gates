package github

import (
	"fmt"
	"net/http"
)

// statusFailure は、GitHub API の成功以外の応答を、Lambda Design の「GitHub API の失敗の分類」のレート制限と 5xx の規則に従って一時的な障害に、それ以外を呼び出し側が渡す otherKind に分ける。
func statusFailure(response *http.Response, requestID string, otherKind error) error {
	kind := otherKind
	if response.StatusCode >= http.StatusInternalServerError || isRateLimited(response) {
		kind = ErrTransientFailure
	}
	return fmt.Errorf("%w: status %d, github request id %q", kind, response.StatusCode, requestID)
}

// 規則は Lambda Design の「GitHub API の失敗の分類」に記述する。
func isRateLimited(response *http.Response) bool {
	switch response.StatusCode {
	case http.StatusTooManyRequests:
		return true
	case http.StatusForbidden:
		_, hasRetryAfter := response.Header["Retry-After"]
		return response.Header.Get("X-Ratelimit-Remaining") == "0" || hasRetryAfter
	default:
		return false
	}
}
