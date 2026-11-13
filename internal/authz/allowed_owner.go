package authz

import (
	"fmt"

	"github.com/tmknom/gates/internal/githubid"
	"github.com/tmknom/gates/internal/repository"
)

// OwnerType は、オーナーが Organization かユーザーアカウントかを表す。
// 値の語の根拠は Lambda Design の「GitHub Client」に記述する。
type OwnerType string

const (
	OwnerTypeOrganization OwnerType = "Organization"
	OwnerTypeUser         OwnerType = "User"
)

// AllowedOwner は、固定設定のうち許可オーナーである。
// フィールドを非公開にし、ゼロ値以外は NewAllowedOwner でしか作れないようにする。
type AllowedOwner struct {
	id        githubid.OwnerID
	name      repository.OwnerName
	ownerType OwnerType
}

// NewAllowedOwner は、INV-004 が定める許可オーナーの条件を検証する。
// オーナー名を文字列で受け取るのは、repository.OwnerName のゼロ値が検証を通らずに入らないようにするため。
func NewAllowedOwner(id int64, name string, ownerType OwnerType) (AllowedOwner, error) {
	ownerID, err := githubid.NewOwnerID(id)
	if err != nil {
		return AllowedOwner{}, fmt.Errorf("%w: owner id %w", ErrInvalidAllowedOwner, err)
	}
	ownerName, err := repository.ParseOwnerName(name)
	if err != nil {
		return AllowedOwner{}, fmt.Errorf("%w: %w", ErrInvalidAllowedOwner, err)
	}
	parsedType, err := ParseOwnerType(string(ownerType))
	if err != nil {
		return AllowedOwner{}, err
	}
	return AllowedOwner{id: ownerID, name: ownerName, ownerType: parsedType}, nil
}

// ParseOwnerType は、許可オーナーの種別を大文字小文字を区別して検証する。
// NewAllowedOwner と別に公開するのは、種別の誤りを Owner ID とオーナー名の誤りと独立に検証できるようにするため。
func ParseOwnerType(s string) (OwnerType, error) {
	ownerType := OwnerType(s)
	if ownerType != OwnerTypeOrganization && ownerType != OwnerTypeUser {
		return "", fmt.Errorf("%w: owner type must be %s or %s", ErrInvalidAllowedOwner, OwnerTypeOrganization, OwnerTypeUser)
	}
	return ownerType, nil
}

// IsZero は、NewAllowedOwner を経ていないゼロ値かを返す。
// NewAllowedOwner はゼロ値の Owner ID を持つ値を作らないため、Owner ID がゼロ値の値はゼロ値に限る。
func (o AllowedOwner) IsZero() bool {
	return o.id.IsZero()
}

// ID は、許可オーナーの不変 Owner ID を返す。
func (o AllowedOwner) ID() githubid.OwnerID {
	return o.id
}

// Name は、許可オーナー名を返す。
func (o AllowedOwner) Name() repository.OwnerName {
	return o.name
}

// Type は、許可オーナーの種別を返す。
func (o AllowedOwner) Type() OwnerType {
	return o.ownerType
}
