package github

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tmknom/gates/internal/authz"
)

// 値の理由は Lambda Design の「GitHub Client」と「Call Time Budget」に記述する。
const (
	installationTimeout   = 5 * time.Second
	installationSizeLimit = 64 * 1024
)

// installationResponse は、Installation の応答のうち authz.Installation に写すフィールドである。
type installationResponse struct {
	ID      int64 `json:"id"`
	Account struct {
		ID int64 `json:"id"`
	} `json:"account"`
	// キーの欠落と null を区別するため、復号前の値で受け取る
	SuspendedAt jsontext.Value `json:"suspended_at"`
}

// ResolveInstallation は、許可オーナーの種別に応じたエンドポイントから、許可オーナーの Installation を取得する。
func (c *Client) ResolveInstallation(ctx context.Context, appJWT AppJWT, owner authz.AllowedOwner) (authz.Installation, error) {
	ctx, cancel := context.WithTimeout(ctx, installationTimeout)
	defer cancel()

	body, requestID, err := c.fetchInstallation(ctx, appJWT, owner)
	if err != nil {
		return authz.Installation{}, err
	}
	installation, err := decodeInstallation(body)
	if err != nil {
		return authz.Installation{}, fmt.Errorf("%w: %w, github request id %q", authz.ErrInstallationUnavailable, err, requestID)
	}
	return installation, nil
}

// fetchInstallation は、Installation の応答のうち 200 の本文と、GitHub の request ID を返す。
func (c *Client) fetchInstallation(ctx context.Context, appJWT AppJWT, owner authz.AllowedOwner) ([]byte, string, error) {
	response, err := c.httpClient.Do(c.newRequest(ctx, http.MethodGet, installationPath(owner), appJWT.redactedValue))
	if err != nil {
		// 通信の error は要求の URL（許可オーナー名を含む）を文言に含めるため包まない
		return nil, "", fmt.Errorf("%w: request failed", ErrTransientFailure)
	}
	// 読み取りの結果は本文の読み取りで確かめており、Close の失敗は結果に影響しない
	defer func() { _ = response.Body.Close() }()

	requestID := response.Header.Get("X-GitHub-Request-Id")
	if response.StatusCode != http.StatusOK {
		return nil, "", statusFailure(response, requestID)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, installationSizeLimit+1))
	if err != nil {
		return nil, "", fmt.Errorf("%w: response body cannot be read, github request id %q", ErrTransientFailure, requestID)
	}
	if len(body) > installationSizeLimit {
		return nil, "", fmt.Errorf("%w: response exceeds %d bytes, github request id %q", authz.ErrInstallationUnavailable, installationSizeLimit, requestID)
	}
	return body, requestID, nil
}

// 許可オーナー名は Lambda Design の「名前の文字種と比較規則」の文字種を満たし、パスの要素としてエスケープを要しない。
func installationPath(owner authz.AllowedOwner) string {
	switch owner.Type() {
	case authz.OwnerTypeOrganization:
		return "/orgs/" + owner.Name().String() + "/installation"
	case authz.OwnerTypeUser:
		return "/users/" + owner.Name().String() + "/installation"
	default:
		// NewAllowedOwner が種別を二つの値に限るため、ここに来るのは NewAllowedOwner を経ないゼロ値だけである
		panic(fmt.Sprintf("allowed owner type %q is neither %s nor %s", owner.Type(), authz.OwnerTypeOrganization, authz.OwnerTypeUser))
	}
}

func decodeInstallation(body []byte) (authz.Installation, error) {
	var response installationResponse
	if err := json.Unmarshal(body, &response); err != nil {
		// 復号の error は本文の値を文言に含め得るため使わない
		return authz.Installation{}, errors.New("response is not an installation")
	}
	suspendedAt, err := readSuspendedAt(response.SuspendedAt)
	if err != nil {
		return authz.Installation{}, err
	}
	return authz.NewInstallation(response.ID, response.Account.ID, suspendedAt)
}

// readSuspendedAt は、Lambda Design の「GitHub Client」の停止状態の読み取りに従い、
// null を停止していない Installation のゼロ値の時刻として、時刻を停止した時刻として読む。
func readSuspendedAt(value jsontext.Value) (time.Time, error) {
	if len(value) == 0 {
		return time.Time{}, errors.New("suspended_at is missing")
	}
	if value.Kind() == 'n' {
		return time.Time{}, nil
	}
	var suspendedAt time.Time
	if err := json.Unmarshal(value, &suspendedAt); err != nil || suspendedAt.IsZero() {
		return time.Time{}, errors.New("suspended_at is neither null nor a time")
	}
	return suspendedAt, nil
}
