// Package awsapi は、AWS STS で IAM Role を引き受け、AWS 一時認証情報で Lambda を Invoke する。
package awsapi

import "fmt"

// InvokeResult は、Lambda Invoke API が返した Invoke の結果である。
type InvokeResult struct {
	StatusCode int
	// FunctionError は X-Amz-Function-Error の値で、ヘッダーがなければ空である。
	FunctionError string
	Payload       []byte
}

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
