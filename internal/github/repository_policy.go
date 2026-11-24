package github

import (
	"context"
	"encoding/base64"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/tmknom/gates/internal/repository"
	"github.com/tmknom/gates/internal/repositorypolicy"
)

// 値の理由は Lambda Design の「GitHub Client」と「Call Time Budget」に記述する。
const (
	repositoryPolicyTimeout   = 5 * time.Second
	repositoryPolicySizeLimit = 2 * 1024 * 1024
)

// FetchRepositoryPolicy は、ポリシー読み取りトークンで、対象の既定ブランチのリポジトリポリシーのパスを取得する。
// 応答の分類は Lambda Design の「リポジトリポリシーの取得の応答の分類」に従う。
func (c *Client) FetchRepositoryPolicy(ctx context.Context, token PolicyReadToken, fullName repository.FullName) (repositorypolicy.FetchResult, error) {
	// 要求を送らない理由は Lambda Design の「リポジトリポリシーの取得の応答の分類」に記述する
	if name := fullName.RepositoryName(); name == "." || name == ".." {
		return repositorypolicy.FetchResult{}, fmt.Errorf("%w: repository name is a dot segment", ErrInternalFailure)
	}
	ctx, cancel := context.WithTimeout(ctx, repositoryPolicyTimeout)
	defer cancel()

	endpoint := repositoryPolicyEndpoint(fullName.OwnerName().String(), fullName.RepositoryName())
	response, err := c.httpClient.Do(c.newRequestTo(ctx, http.MethodGet, endpoint, token))
	if err != nil {
		// 通信の error は要求の URL（完全名を含む）を文言に含めるため包まない
		return repositorypolicy.FetchResult{}, fmt.Errorf("%w: request failed", ErrTransientFailure)
	}
	// 読み取りの結果は本文の読み取りで確かめており、Close の失敗は結果に影響しない
	defer func() { _ = response.Body.Close() }()

	requestID := response.Header.Get("X-GitHub-Request-Id")
	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return repositorypolicy.NotFound(), nil
	default:
		return repositorypolicy.FetchResult{}, statusFailure(response, requestID, ErrInternalFailure)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, repositoryPolicySizeLimit+1))
	if err != nil {
		return repositorypolicy.FetchResult{}, fmt.Errorf("%w: response body cannot be read, github request id %q", ErrTransientFailure, requestID)
	}
	if len(body) > repositoryPolicySizeLimit {
		return repositorypolicy.FetchResult{}, fmt.Errorf("%w: response exceeds %d bytes, github request id %q", ErrInternalFailure, repositoryPolicySizeLimit, requestID)
	}
	result, err := decodeContents(body)
	if err != nil {
		return repositorypolicy.FetchResult{}, fmt.Errorf("%w: %w, github request id %q", ErrInternalFailure, err, requestID)
	}
	return result, nil
}

// repositoryPolicyEndpoint は、オーナー名とリポジトリ名をそれぞれパスの 1 区画としてエスケープし、ref を付けない。
// 形式検証を通った名前はエスケープを要しない文字だけからなるが、パスの区画を形式検証の文字種に依存させないため、エスケープを省かない。
func repositoryPolicyEndpoint(ownerName, repositoryName string) *url.URL {
	const contents = "/contents/" + repositorypolicy.Path
	return &url.URL{
		Path:    "/repos/" + ownerName + "/" + repositoryName + contents,
		RawPath: "/repos/" + url.PathEscape(ownerName) + "/" + url.PathEscape(repositoryName) + contents,
	}
}

// contentsResponse は、Contents API のファイルの応答のうち、分類に使うフィールドである。
// フィールドごとに読むかどうかが type と size で決まるため、復号前の値で受け取る。
type contentsResponse struct {
	Type     jsontext.Value `json:"type"`
	Size     jsontext.Value `json:"size"`
	Encoding jsontext.Value `json:"encoding"`
	Content  jsontext.Value `json:"content"`
}

// decodeContents は、200 の本文を取得の結果に分類する。返す error の文言は、本文の値を含めない。
// 本文の最初の値の種類で復号先を選び、どちらの復号も本文全体の構文、重複した名前、UTF-8 を確かめる。
func decodeContents(body []byte) (repositorypolicy.FetchResult, error) {
	switch jsontext.Value(body).Kind() {
	case '[':
		if err := json.Unmarshal(body, new([]jsontext.Value)); err != nil {
			return repositorypolicy.FetchResult{}, errors.New("response is not a JSON array")
		}
		return repositorypolicy.NotFile(), nil
	case '{':
	default:
		return repositorypolicy.FetchResult{}, errors.New("response is neither a JSON object nor an array")
	}
	var response contentsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return repositorypolicy.FetchResult{}, errors.New("response is not a JSON object")
	}

	contentType, ok := readField[string](response.Type)
	if !ok {
		return repositorypolicy.FetchResult{}, errors.New("type is not a string")
	}
	if contentType != "file" {
		return repositorypolicy.NotFile(), nil
	}
	size, ok := readField[uint64](response.Size)
	if !ok {
		return repositorypolicy.FetchResult{}, errors.New("size is not a non-negative integer")
	}
	if size > repositorypolicy.MaxDocumentBytes {
		return repositorypolicy.TooLarge(), nil
	}
	if encoding, ok := readField[string](response.Encoding); !ok || encoding != "base64" {
		return repositorypolicy.FetchResult{}, errors.New("encoding is not base64")
	}
	content, ok := readField[string](response.Content)
	if !ok {
		return repositorypolicy.FetchResult{}, errors.New("content is not a string")
	}
	// GitHub は content を改行で区切る。StdEncoding は \r と \n を読み飛ばす
	document, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return repositorypolicy.FetchResult{}, errors.New("content is not base64")
	}
	if uint64(len(document)) != size {
		return repositorypolicy.FetchResult{}, errors.New("decoded content length does not match size")
	}
	return repositorypolicy.Document(string(document)), nil
}
