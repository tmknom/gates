package awsapi

import "fmt"

// APIError は、AWS が返したエラー応答のエラーコードと HTTP ステータスである。
// 応答の Message は要求の一部を含み得るため持たない（GitHub Actions Design の Failure Output）。
type APIError struct {
	// Code は AWS のエラーコードで、応答から読み取れなければ空である。
	Code       string
	HTTPStatus int
}

// Error は、エラーコードと HTTP ステータスを示す。
func (e *APIError) Error() string {
	if e.Code == "" {
		return fmt.Sprintf("HTTP status %d without an AWS error code", e.HTTPStatus)
	}
	return fmt.Sprintf("AWS error code %q, HTTP status %d", e.Code, e.HTTPStatus)
}
