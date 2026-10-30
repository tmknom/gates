package request

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"

	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/repository"
)

const (
	maxPayloadBytes = 16 * 1024
	maxRepositories = 500
)

// payload は、Repository Request 形式の Payload を復号する入れ物である。
// encoding/json/v2 は null をゼロ値として復号するため、欠落と null を区別できない。
// どのフィールドもゼロ値が正当な値と重ならず、欠落も null も入力不正であるため、区別せずにゼロ値で判定する。
type payload struct {
	Version      int               `json:"version"`
	IDToken      string            `json:"id_token"`
	Repositories rawFullNames      `json:"repositories"`
	Permissions  map[string]string `json:"permissions"`
}

// fieldErrors は、値を期待する型に復号できないときに返す sentinel error を、Payload のフィールド名で引く。
var fieldErrors = map[string]error{
	"version":      ErrInvalidVersion,
	"id_token":     ErrInvalidIDTokenField,
	"repositories": ErrInvalidRepositories,
	"permissions":  ErrInvalidPermissions,
}

// Validate は、Payload を形式検証し、未検証の GATES 用 OIDC JWT と発行要求に分けて返す。
// 値を読み取れない形だけを入力不正にし、Permissions が許可条件に合うかは判定しない。
// 返す error はどれも入力不正である。sentinel error は、どの規則で拒否したかを区別するために分ける。
func Validate(b []byte) (RawIDToken, TokenRequest, error) {
	if len(b) > maxPayloadBytes {
		return RawIDToken{}, TokenRequest{}, fmt.Errorf("%w: must be at most %d bytes", ErrPayloadTooLarge, maxPayloadBytes)
	}
	p, err := decode(b)
	if err != nil {
		return RawIDToken{}, TokenRequest{}, err
	}

	if p.Version != 1 {
		return RawIDToken{}, TokenRequest{}, ErrInvalidVersion
	}
	if p.IDToken == "" {
		return RawIDToken{}, TokenRequest{}, ErrInvalidIDTokenField
	}
	repositories, err := p.Repositories.parse()
	if err != nil {
		return RawIDToken{}, TokenRequest{}, err
	}
	permissions, err := permission.NewPermissions(p.Permissions)
	if err != nil {
		return RawIDToken{}, TokenRequest{}, fmt.Errorf("%w: %w", ErrInvalidPermissions, err)
	}

	req := TokenRequest{
		repositories: repositories,
		permissions:  permissions,
	}
	// p のフィールドを指すと、トークンが復号した Payload 全体を保持し続けるため、値を写してから指す
	idToken := p.IDToken
	return RawIDToken{value: &idToken}, req, nil
}

// decode は、重複したキーと不正な UTF-8 を拒否し、フィールド名の大文字小文字を区別する encoding/json/v2 の既定に、未知のフィールドの拒否を加えて復号する。
// encoding/json/v2 のエラー文言は入力の断片を含むため包まず、種類だけを判定して sentinel error を返す。
func decode(b []byte) (*payload, error) {
	// トップレベルの null は struct に復号するとゼロ値になり version の不正と区別できないため、ポインタに復号して nil で見分ける
	var p *payload
	err := json.Unmarshal(b, &p, json.RejectUnknownMembers(true))
	if err == nil {
		if p == nil {
			return nil, fmt.Errorf("%w: payload must be a JSON object", ErrMalformedPayload)
		}
		return p, nil
	}

	if errors.Is(err, json.ErrUnknownName) {
		return nil, ErrUnknownField
	}
	semantic, ok := errors.AsType[*json.SemanticError](err)
	if !ok {
		return nil, ErrMalformedPayload
	}
	// JSON Pointer の先頭のトークンだけを見る。2 番目以降には permissions のキーのような利用者の入力が現れるため
	name, _, _ := strings.Cut(strings.TrimPrefix(string(semantic.JSONPointer), "/"), "/")
	if fieldErr, ok := fieldErrors[name]; ok {
		// 型の不一致だけでなく、int に収まらない数値もここに来る
		return nil, fmt.Errorf("%w: cannot be decoded as the expected type", fieldErr)
	}
	return nil, fmt.Errorf("%w: payload must be a JSON object", ErrMalformedPayload)
}

// 解析は復号の時点で行わない。行うと、完全名の形式の誤りが decode の文言に一本化されて repository.ErrInvalidFullName を失い、ほかのフィールドの検証より先に判定される。
type rawFullNames []string

// parse は、件数、完全名の形式、大文字小文字を区別しない重複を検証する。
// 重複は FullName.Equal の総当たりで判定し、比較規則の写しを持たない。件数を先に maxRepositories 以下に限るため、総当たりの回数にも上限がある。
// 不正な要素は最初の 1 件で返す。どの要素で拒否しても入力不正という結果は変わらず、失敗応答も要素を示さないため。
func (names rawFullNames) parse() ([]repository.FullName, error) {
	if len(names) == 0 || len(names) > maxRepositories {
		return nil, fmt.Errorf("%w: must have 1 to %d items", ErrInvalidRepositories, maxRepositories)
	}
	repositories := make([]repository.FullName, 0, len(names))
	for i, name := range names {
		f, err := repository.ParseFullName(name)
		if err != nil {
			return nil, fmt.Errorf("%w: repositories[%d]: %w", ErrInvalidRepositories, i, err)
		}
		for j, prev := range repositories {
			if f.Equal(prev) {
				return nil, fmt.Errorf("%w: repositories[%d] duplicates repositories[%d]", ErrInvalidRepositories, i, j)
			}
		}
		repositories = append(repositories, f)
	}
	return repositories, nil
}
