package github

import (
	"fmt"
	"net/http"

	"github.com/tmknom/gates/internal/authz"
)

// statusFailure は、GitHub API の成功以外の応答を、Lambda Design の「GitHub API の失敗の分類」に従って分類する。
// 発行 API の 422 は呼び出し側が先に分ける。
func statusFailure(response *http.Response, requestID string) error {
	kind := authz.ErrInstallationUnavailable
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
