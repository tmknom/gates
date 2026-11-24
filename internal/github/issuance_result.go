package github

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"slices"
	"time"

	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/repository"
)

// IssuanceResult は、発行応答から項目ごとに独立に読み取った発行結果である。照合の前の値で、不変条件を持たない。
// 各アクセサの第 2 戻り値は、その項目を応答から読み取れたかを表す。
// 読み取れなかった項目を照合がどう扱うかは、Domain Model の「Issuance Result」に記述する。
type IssuanceResult struct {
	// Domain Model の発行結果の項目ではない。照合失敗の調査に使うため、本文を読めたかによらず添える
	requestID           string
	token               responseField[InstallationAccessToken]
	expiresAt           responseField[time.Time]
	permissions         responseField[permission.Permissions]
	repositorySelection responseField[RepositorySelection]
	repositories        responseField[[]EffectiveRepository]
	policyReadToken     responseField[PolicyReadToken]
	readableFilePaths   responseField[[]string]
}

// RepositorySelection は、対象が選択したリポジトリに限られるか、全リポジトリかを表す対象の選び方である。
type RepositorySelection string

// RepositorySelectionSelected は、選択したリポジトリに限る対象の選び方である。
const RepositorySelectionSelected RepositorySelection = "selected"

// responseField は、発行応答の項目の値と、その項目を読み取れたかの組である。
// 値と読み取れたかを別々のフィールドに持つと、片方だけを代入し忘れても気付けないため、一組で代入する。
type responseField[T any] struct {
	value T
	read  bool
}

func newResponseField[T any](value T, read bool) responseField[T] {
	return responseField[T]{value: value, read: read}
}

// GitHubRequestID は、発行応答の GitHub の request ID を返す。Lambda の相関 ID（Request ID）とは別の値である。
func (r IssuanceResult) GitHubRequestID() string {
	return r.requestID
}

// Token は、Installation Access Token を返す。
func (r IssuanceResult) Token() (InstallationAccessToken, bool) {
	return r.token.value, r.token.read
}

// ExpiresAt は、GitHub が定めた有効期限を返す。
func (r IssuanceResult) ExpiresAt() (time.Time, bool) {
	return r.expiresAt.value, r.expiresAt.read
}

// EffectivePermissions は、実効権限を返す。
func (r IssuanceResult) EffectivePermissions() (permission.Permissions, bool) {
	return r.permissions.value, r.permissions.read
}

// RepositorySelection は、対象の選び方を返す。
func (r IssuanceResult) RepositorySelection() (RepositorySelection, bool) {
	return r.repositorySelection.value, r.repositorySelection.read
}

// EffectiveRepositories は、実効対象を応答の順序のまま返す。
// 呼び出し元が並べ替えや書き換えをしても、発行結果が変わらないように写す。
func (r IssuanceResult) EffectiveRepositories() ([]EffectiveRepository, bool) {
	return slices.Clone(r.repositories.value), r.repositories.read
}

// PolicyReadToken は、ポリシー読み取りトークンを返す。ポリシー読み取りトークンの発行応答でだけ読み取る。
func (r IssuanceResult) PolicyReadToken() (PolicyReadToken, bool) {
	return r.policyReadToken.value, r.policyReadToken.read
}

// ReadableFilePaths は、読めるファイルのパスを応答の順序のまま返す。ポリシー読み取りトークンの発行応答でだけ読み取る。
// 呼び出し元が書き換えても、発行結果が変わらないように写す。
func (r IssuanceResult) ReadableFilePaths() ([]string, bool) {
	return slices.Clone(r.readableFilePaths.value), r.readableFilePaths.read
}

// EffectiveRepository は、発行結果に含まれる各リポジトリである。
type EffectiveRepository struct {
	repositoryID int64
	ownerID      int64
	fullName     repository.FullName
}

// RepositoryID は、リポジトリの不変 Repository ID を返す。
func (e EffectiveRepository) RepositoryID() int64 {
	return e.repositoryID
}

// OwnerID は、リポジトリを所有するオーナーの不変 Owner ID を返す。
func (e EffectiveRepository) OwnerID() int64 {
	return e.ownerID
}

// FullName は、GitHub 上の表記の完全名を返す。
func (e EffectiveRepository) FullName() repository.FullName {
	return e.fullName
}

// issuanceResponse は、発行応答のうち発行結果に写すフィールドである。
// フィールドごとに独立に読むため、復号前の値で受け取る。
type issuanceResponse struct {
	Token               jsontext.Value `json:"token"`
	ExpiresAt           jsontext.Value `json:"expires_at"`
	Permissions         jsontext.Value `json:"permissions"`
	RepositorySelection jsontext.Value `json:"repository_selection"`
	Repositories        jsontext.Value `json:"repositories"`
	SingleFilePaths     jsontext.Value `json:"single_file_paths"`
}

// decodeIssuanceResult は、Lambda Design の「GitHub Client」の発行応答の読み取りに従い、利用者に返すトークンの発行応答の本文を発行結果にする。
func decodeIssuanceResult(body []byte) IssuanceResult {
	response, ok := decodeIssuanceResponse(body)
	if !ok {
		return IssuanceResult{}
	}
	result := response.sharedFields()
	result.token = newResponseField(readToken(response.Token))
	return result
}

// decodePolicyReadIssuanceResult は、ポリシー読み取りトークンの発行応答の本文を発行結果にする。
// トークン文字列をポリシー読み取りトークンとしてだけ読み、利用者に返すトークンとしては読まない。
func decodePolicyReadIssuanceResult(body []byte) IssuanceResult {
	response, ok := decodeIssuanceResponse(body)
	if !ok {
		return IssuanceResult{}
	}
	result := response.sharedFields()
	result.policyReadToken = newResponseField(readPolicyReadToken(response.Token))
	result.readableFilePaths = newResponseField(readFilePaths(response.SingleFilePaths))
	return result
}

// decodeIssuanceResponse は、本文を JSON のオブジェクトとして解釈できなければ、どの項目も読み取らないよう false を返す。
func decodeIssuanceResponse(body []byte) (issuanceResponse, bool) {
	var response issuanceResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return issuanceResponse{}, false
	}
	return response, true
}

// sharedFields は、どちらの発行応答でも同じ規則で読む、トークン以外の項目を読み取る。
func (r issuanceResponse) sharedFields() IssuanceResult {
	return IssuanceResult{
		expiresAt:           newResponseField(readField[time.Time](r.ExpiresAt)),
		permissions:         newResponseField(readPermissions(r.Permissions)),
		repositorySelection: newResponseField(readField[RepositorySelection](r.RepositorySelection)),
		repositories:        newResponseField(readRepositories(r.Repositories)),
	}
}

// readField は、キーの欠落と null を読み取れなかったものとし、それ以外を T として読む。
// encoding/json/v2 は null を復号の失敗にせず、ゼロ値のまま返すため、先に分ける。
func readField[T any](value jsontext.Value) (T, bool) {
	var field T
	if len(value) == 0 || value.Kind() == 'n' {
		return field, false
	}
	if err := json.Unmarshal(value, &field); err != nil {
		return field, false
	}
	return field, true
}

func readToken(value jsontext.Value) (InstallationAccessToken, bool) {
	token, ok := readTokenString(value)
	if !ok {
		return InstallationAccessToken{}, false
	}
	return InstallationAccessToken{value: &token}, true
}

func readPolicyReadToken(value jsontext.Value) (PolicyReadToken, bool) {
	token, ok := readTokenString(value)
	if !ok {
		return PolicyReadToken{}, false
	}
	return PolicyReadToken{value: &token}, true
}

// readTokenString の規則は Lambda Design の「GitHub Client」に記述する。
func readTokenString(value jsontext.Value) (string, bool) {
	token, ok := readField[string](value)
	if !ok || token == "" {
		return "", false
	}
	return token, true
}

// readPermissions は、実効権限を要求の Permissions と同じ文字種の規則で読む。
func readPermissions(value jsontext.Value) (permission.Permissions, bool) {
	levels, ok := readField[map[string]string](value)
	if !ok {
		return permission.Permissions{}, false
	}
	permissions, err := permission.NewPermissions(levels)
	if err != nil {
		return permission.Permissions{}, false
	}
	return permissions, true
}

// effectiveRepositoryResponse は、発行応答の repositories の要素のうち実効対象に写すフィールドである。
// キーの欠落と null をゼロ値と区別するため、ポインタで受け取る。
type effectiveRepositoryResponse struct {
	ID    *int64 `json:"id"`
	Owner *struct {
		ID *int64 `json:"id"`
	} `json:"owner"`
	FullName *string `json:"full_name"`
}

// readRepositories は、どれかの要素を実効対象として読めなければ、集合全体を読み取れなかったものとする。
func readRepositories(value jsontext.Value) ([]EffectiveRepository, bool) {
	elements, ok := readField[[]effectiveRepositoryResponse](value)
	if !ok {
		return nil, false
	}
	repositories := make([]EffectiveRepository, 0, len(elements))
	for _, element := range elements {
		if element.ID == nil || element.Owner == nil || element.Owner.ID == nil || element.FullName == nil {
			return nil, false
		}
		fullName, err := repository.ParseFullName(*element.FullName)
		if err != nil {
			return nil, false
		}
		repositories = append(repositories, EffectiveRepository{repositoryID: *element.ID, ownerID: *element.Owner.ID, fullName: fullName})
	}
	return repositories, true
}

// readFilePaths は、文字列の配列だけを読み取る。null の要素を空文字列と区別するため、要素をポインタで受け取る。
func readFilePaths(value jsontext.Value) ([]string, bool) {
	elements, ok := readField[[]*string](value)
	if !ok {
		return nil, false
	}
	paths := make([]string, 0, len(elements))
	for _, element := range elements {
		if element == nil {
			return nil, false
		}
		paths = append(paths, *element)
	}
	return paths, true
}
