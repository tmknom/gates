// Package response は、Lambda の Invoke の結果を判定し、成功応答から Installation Access Token と Output の値を取り出す。
// 失敗の文言は、段階を示す固定の語で始め、応答の本文、errorMessage、トークン、文字列でない値、`##[` を含む値を含めない（GitHub Actions Design の Failure Output）。
package response

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"

	"github.com/tmknom/gates/actions/issue-token/internal/awsapi"
	"github.com/tmknom/gates/actions/issue-token/internal/credential"
)

// envelope は、成功応答と失敗応答に共通する最上位の項目である。値の型は、使う段階で確かめる。
type envelope struct {
	OK           jsontext.Value `json:"ok"`
	Token        jsontext.Value `json:"token"`
	ExpiresAt    jsontext.Value `json:"expires_at"`
	Repositories jsontext.Value `json:"repositories"`
	Permissions  jsontext.Value `json:"permissions"`
	RequestID    jsontext.Value `json:"request_id"`
	Error        jsontext.Value `json:"error"`
}

// Judge は、Invoke の結果を SDK のエラー、StatusCode、FunctionError、JSON と ok、ok の値、token の順に判定する。
// 取り出すトークンの条件は GitHub Actions Design の Processing Flow に従う。
// invokeErr は分岐しない。awsapi が文言から応答の Message と URL を除いている。
func Judge(result awsapi.InvokeResult, invokeErr error) (Success, error) {
	if invokeErr != nil {
		return Success{}, fmt.Errorf("the Lambda Invoke failed: %w", invokeErr)
	}
	if result.StatusCode != 200 {
		return Success{}, fmt.Errorf("the Lambda Invoke returned StatusCode %d", result.StatusCode)
	}
	if result.FunctionError != "" {
		return Success{}, functionFailure(result)
	}

	var body envelope
	if err := json.Unmarshal(result.Payload, &body); err != nil {
		return Success{}, fmt.Errorf("the Lambda response is not valid JSON (%d bytes)", len(result.Payload))
	}
	switch body.OK.Kind() {
	case 't':
	case 'f':
		return Success{}, failureResponse(body)
	default:
		return Success{}, fmt.Errorf("the Lambda response has no boolean ok (%d bytes)", len(result.Payload))
	}

	token, ok := stringOf(body.Token)
	if !ok || token == "" || strings.ContainsFunc(token, isASCIIControl) {
		return Success{}, errors.New("the success response has no usable token" + requestIDSuffix(body.RequestID))
	}
	return Success{
		token:        credential.NewInstallationAccessToken(token),
		expiresAt:    body.ExpiresAt,
		repositories: body.Repositories,
		permissions:  body.Permissions,
		requestID:    body.RequestID,
	}, nil
}

// functionFailure は、FunctionError を段階の文言にする。FunctionError と errorType は displayable を満たすときだけ含め、errorMessage は含めない。
func functionFailure(result awsapi.InvokeResult) error {
	message := "the Lambda function returned FunctionError"
	if displayable(result.FunctionError) {
		message += fmt.Sprintf(" %q", result.FunctionError)
	}
	var payload struct {
		ErrorType jsontext.Value `json:"errorType"`
	}
	if err := json.Unmarshal(result.Payload, &payload); err != nil {
		return errors.New(message)
	}
	if errorType, ok := displayString(payload.ErrorType); ok {
		message += fmt.Sprintf(" with errorType %q", errorType)
	}
	return errors.New(message)
}

// failureResponse は、ok が false の応答を、displayString を満たす項目だけを並べた文言にする。
// message は Lambda が固定の文言で返すため含める。
func failureResponse(body envelope) error {
	var detail struct {
		Code    jsontext.Value `json:"code"`
		Message jsontext.Value `json:"message"`
	}
	// error がオブジェクトでなければ、文字列の項目がないものとして扱う
	_ = json.Unmarshal(body.Error, &detail)

	var parts []string
	if code, ok := displayString(detail.Code); ok {
		parts = append(parts, fmt.Sprintf("error code %q", code))
	}
	if message, ok := displayString(detail.Message); ok {
		parts = append(parts, fmt.Sprintf("message %q", message))
	}
	if requestID, ok := displayString(body.RequestID); ok {
		parts = append(parts, fmt.Sprintf("request id %q", requestID))
	}
	if len(parts) == 0 {
		return errors.New("GATES returned a failure response")
	}
	return errors.New("GATES returned a failure response: " + strings.Join(parts, ", "))
}

// isASCIIControl は、ASCII の制御文字（0x00 から 0x1f と 0x7f）かを返す。
func isASCIIControl(r rune) bool {
	return r < 0x20 || r == 0x7f
}
