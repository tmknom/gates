package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/githubid"
	"github.com/tmknom/gates/internal/nonzero"
	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/repository"
)

// 値の理由は Lambda Design の「GitHub Client」と「Call Time Budget」に記述する。
const (
	issuanceTimeout   = 10 * time.Second
	issuanceSizeLimit = 16 * 1024 * 1024
)

// Issue は、発行許可の対象と権限を明示して Installation Access Token を発行させ、発行応答を発行結果にする。
// 201 の応答は、項目を読み取れたかどうかにかかわらず発行結果にし、照合を Token Issuer に任せる。
func (c *Client) Issue(ctx context.Context, appJWT AppJWT, grant authz.TokenGrant) (IssuanceResult, error) {
	// ゼロ値は対象も権限も持たず、そのまま送ると省略として Installation の全対象と App の全権限を継承するため、要求を送らない
	nonzero.Require(grant)
	body, requestID, err := c.postIssuance(ctx, appJWT, grant.InstallationID(), issuanceRequestBody(grant.Repositories(), grant.Permissions()))
	if err != nil {
		return IssuanceResult{}, err
	}
	result := decodeIssuanceResult(body)
	result.requestID = requestID
	return result, nil
}

// IssuePolicyReadToken は、ポリシー読み取り許可の対象と single_file:read を明示してポリシー読み取りトークンを発行させ、発行応答を発行結果にする。
// 発行応答の扱いと失敗の分類は Issue と同じである。
func (c *Client) IssuePolicyReadToken(ctx context.Context, appJWT AppJWT, grant authz.PolicyReadGrant) (IssuanceResult, error) {
	// ゼロ値は対象を持たず、そのまま送ると省略として Installation の全対象を継承するため、要求を送らない
	nonzero.Require(grant)
	body, requestID, err := c.postIssuance(ctx, appJWT, grant.InstallationID(), issuanceRequestBody(grant.Repositories(), grant.Permissions()))
	if err != nil {
		return IssuanceResult{}, err
	}
	result := decodePolicyReadIssuanceResult(body)
	result.requestID = requestID
	return result, nil
}

// postIssuance は、発行応答のうち 201 の本文を、GitHub の request ID とともに返す。
// 上限を超える本文は、どの項目も読まない発行結果にするため、error にせず空の本文として返す。
func (c *Client) postIssuance(ctx context.Context, appJWT AppJWT, installationID githubid.InstallationID, body string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(ctx, issuanceTimeout)
	defer cancel()

	request := c.newRequest(ctx, http.MethodPost, issuancePath(installationID), appJWT)
	request.Header.Set("Content-Type", "application/json")
	request.Body = io.NopCloser(strings.NewReader(body))
	request.ContentLength = int64(len(body))

	response, err := c.httpClient.Do(request)
	if err != nil {
		// 通信の error は要求の URL（Installation ID を含む）を文言に含めるため包まない
		return nil, "", fmt.Errorf("%w: request failed", ErrTransientFailure)
	}
	// 読み取りの結果は本文の読み取りで確かめており、Close の失敗は結果に影響しない
	defer func() { _ = response.Body.Close() }()

	requestID := response.Header.Get("X-GitHub-Request-Id")
	switch response.StatusCode {
	case http.StatusCreated:
	case http.StatusUnprocessableEntity:
		return nil, "", fmt.Errorf("%w: status %d, github request id %q", ErrIssuanceRejected, response.StatusCode, requestID)
	default:
		return nil, "", statusFailure(response, requestID, authz.ErrInstallationUnavailable)
	}
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, issuanceSizeLimit+1))
	if err != nil {
		return nil, "", fmt.Errorf("%w: response body cannot be read, github request id %q", ErrTransientFailure, requestID)
	}
	if len(responseBody) > issuanceSizeLimit {
		return nil, requestID, nil
	}
	return responseBody, requestID, nil
}

func issuancePath(installationID githubid.InstallationID) string {
	return "/app/installations/" + strconv.FormatInt(installationID.Int64(), 10) + "/access_tokens"
}

// issuanceRequestBody は、対象のリポジトリ名と権限を、どちらも省略せずに JSON にする。
// リポジトリ名は Lambda Design の「名前の文字種と比較規則」、権限名と水準は「権限名と水準の文字種」を満たし、JSON のエスケープを要しない。
// encoding/json を使わないのは、失敗しない Marshal の error の分岐を作らないためである。
func issuanceRequestBody(repositories []repository.FullName, permissions permission.Permissions) string {
	var b strings.Builder
	b.WriteString(`{"repositories":[`)
	for i, fullName := range repositories {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`"` + fullName.RepositoryName() + `"`)
	}
	b.WriteString(`],"permissions":{`)
	separator := ""
	for name, level := range permissions.Entries() {
		b.WriteString(separator + `"` + name + `":"` + level + `"`)
		separator = ","
	}
	b.WriteString("}}")
	return b.String()
}
