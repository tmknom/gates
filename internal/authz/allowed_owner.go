package authz

import (
	"fmt"

	"github.com/tmknom/gates/internal/repository"
)

// OwnerType は、Owner が Organization かユーザーアカウントかを表す。
// 値の語の根拠は Domain Model の「Fixed Configuration」に記述する。
type OwnerType string

const (
	OwnerTypeOrganization OwnerType = "Organization"
	OwnerTypeUser         OwnerType = "User"
)

// AllowedOwner は、固定設定のうち許可 Owner である。
// フィールドを非公開にし、ゼロ値以外は NewAllowedOwner でしか作れないようにする。
type AllowedOwner struct {
	id        int64
	name      repository.OwnerName
	ownerType OwnerType
}

// NewAllowedOwner は、Domain Model の「Invariants」が定める許可 Owner の条件を検証する。
// Owner 名を文字列で受け取るのは、repository.OwnerName のゼロ値が検証を通らずに入らないようにするため。
func NewAllowedOwner(id int64, name string, ownerType OwnerType) (AllowedOwner, error) {
	if id <= 0 {
		return AllowedOwner{}, fmt.Errorf("%w: owner id must be a positive integer", ErrInvalidAllowedOwner)
	}
	ownerName, err := repository.ParseOwnerName(name)
	if err != nil {
		return AllowedOwner{}, fmt.Errorf("%w: %w", ErrInvalidAllowedOwner, err)
	}
	if ownerType != OwnerTypeOrganization && ownerType != OwnerTypeUser {
		return AllowedOwner{}, fmt.Errorf("%w: owner type must be %s or %s", ErrInvalidAllowedOwner, OwnerTypeOrganization, OwnerTypeUser)
	}
	return AllowedOwner{id: id, name: ownerName, ownerType: ownerType}, nil
}

// ID は、許可 Owner の不変 Owner ID を返す。
func (o AllowedOwner) ID() int64 {
	return o.id
}

// Name は、許可 Owner 名を返す。
func (o AllowedOwner) Name() repository.OwnerName {
	return o.name
}

// Type は、許可 Owner の種別を返す。
func (o AllowedOwner) Type() OwnerType {
	return o.ownerType
}
