// Package response は、Lambda の Invoke の結果を判定し、成功応答から Installation Access Token と Output の値を取り出す。
// 失敗の文言は、段階を示す固定の語で始め、応答の本文、errorMessage、トークン、文字列でない値、`##[` を含む値を含めない（GitHub Actions Design の Failure Output）。
package response

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/awsapi"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/credential"
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

// Success は、トークンを取り出した成功応答である。
// 残りの項目は、トークンのマスクとキャンセルの確認を挟むため、Items が判定する。
type Success struct {
	token                                           credential.InstallationAccessToken
	expiresAt, repositories, permissions, requestID jsontext.Value
}

// Token は、成功応答から取り出したトークンを返す。
func (s Success) Token() credential.InstallationAccessToken {
	return s.token
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

// stringOf は、JSON の文字列の値を返す。文字列でなければ false を返す。
func stringOf(value jsontext.Value) (string, bool) {
	if value.Kind() != '"' {
		return "", false
	}
	var s string
	// 値は全体の復号を通った JSON の文字列のため、復号は失敗しない
	_ = json.Unmarshal(value, &s)
	return s, true
}

// isASCIIControl は、ASCII の制御文字（0x00 から 0x1f と 0x7f）かを返す。
func isASCIIControl(r rune) bool {
	return r < 0x20 || r == 0x7f
}

// legacyCommandPrefix は、Runner が行の途中からも読む旧形式のワークフローコマンドの接頭辞である（actions/runner の ActionCommand.TryParse）。
const legacyCommandPrefix = "##["

// usableItem は、項目の値が空でなく、改行も旧形式のワークフローコマンドの接頭辞も含まないかを返す。
// 条件は GitHub Actions Design の Inputs and Outputs に従う。
func usableItem(s string) bool {
	return s != "" && !strings.ContainsAny(s, "\r\n") && !strings.Contains(s, legacyCommandPrefix)
}

// itemString は、項目が文字列で usableItem を満たすときだけ、その値を返す。
func itemString(value jsontext.Value) (string, bool) {
	s, ok := stringOf(value)
	if !ok || !usableItem(s) {
		return "", false
	}
	return s, true
}

// displayable は、値を失敗の文言に含めてよいかを返す。
// 失敗の文言も注釈としてログの行に出すため、usableItem と同じく旧形式のワークフローコマンドの接頭辞を拒む。
// 改行は注釈の符号化と %q が 1 行に収めるため、拒まない。
func displayable(s string) bool {
	return !strings.Contains(s, legacyCommandPrefix)
}

// displayString は、項目が文字列で displayable を満たすときだけ、その値を返す。
func displayString(value jsontext.Value) (string, bool) {
	s, ok := stringOf(value)
	if !ok || !displayable(s) {
		return "", false
	}
	return s, true
}

// requestIDSuffix は、request_id が displayString を満たすときだけ、文言に添える括弧の部分を返す。
func requestIDSuffix(requestID jsontext.Value) string {
	id, ok := displayString(requestID)
	if !ok {
		return ""
	}
	return fmt.Sprintf(" (request id %q)", id)
}

// Items は、成功応答のトークン以外の項目である。expires_at、repositories、permissions は Output に、request_id はログに出す。
type Items struct {
	ExpiresAt string
	// Repositories は、応答の順のまま , で結ぶ。
	Repositories string
	// Permissions は、権限名のバイト順の昇順に name:level を , で結ぶ。
	Permissions string
	RequestID   string
}

// Items は、request_id、expires_at、repositories、permissions の順に判定し、最初に満たさなかった項目で失敗する。
// 拒否する値の条件は GitHub Actions Design の Inputs and Outputs に従う。
func (s Success) Items() (Items, error) {
	requestID, ok := itemString(s.requestID)
	if !ok {
		return Items{}, errors.New("the success response has no usable request_id")
	}
	unusable := func(item string) error {
		return errors.New("the success response has no usable " + item + requestIDSuffix(s.requestID))
	}

	expiresAt, ok := itemString(s.expiresAt)
	if !ok {
		return Items{}, unusable("expires_at")
	}
	repositories, ok := joinRepositories(s.repositories)
	if !ok {
		return Items{}, unusable("repositories")
	}
	permissions, ok := joinPermissions(s.permissions)
	if !ok {
		return Items{}, unusable("permissions")
	}
	return Items{
		ExpiresAt:    expiresAt,
		Repositories: repositories,
		Permissions:  permissions,
		RequestID:    requestID,
	}, nil
}

// joinRepositories は、1 件以上の配列の各要素が usableItem を満たし、, を含まないときだけ、応答の順に , で結ぶ。
func joinRepositories(value jsontext.Value) (string, bool) {
	var names []string
	if err := json.Unmarshal(value, &names); err != nil || len(names) == 0 {
		return "", false
	}
	for _, name := range names {
		if !usableItem(name) || strings.Contains(name, ",") {
			return "", false
		}
	}
	return strings.Join(names, ","), true
}

// joinPermissions は、1 件以上のオブジェクトの権限名と水準が usableItem を満たし、, と : を含まないときだけ、
// 権限名の昇順に name:level を , で結ぶ。
func joinPermissions(value jsontext.Value) (string, bool) {
	var levels map[string]string
	if err := json.Unmarshal(value, &levels); err != nil || len(levels) == 0 {
		return "", false
	}
	var pairs []string
	for _, name := range slices.Sorted(maps.Keys(levels)) {
		level := levels[name]
		if !usableItem(name) || !usableItem(level) || strings.ContainsAny(name, ",:") || strings.ContainsAny(level, ",:") {
			return "", false
		}
		pairs = append(pairs, name+":"+level)
	}
	return strings.Join(pairs, ","), true
}
