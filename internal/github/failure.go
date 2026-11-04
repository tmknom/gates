package github

import "net/http"

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
