package authz

import "fmt"

// OwnerType は、オーナーが Organization かユーザーアカウントかを表す。
// 値の語の根拠は Lambda Design の「GitHub Client」に記述する。
type OwnerType string

const (
	OwnerTypeOrganization OwnerType = "Organization"
	OwnerTypeUser         OwnerType = "User"
)

// ParseOwnerType は、許可オーナーの種別を大文字小文字を区別して検証する。
// NewAllowedOwner と別に公開するのは、種別の誤りを Owner ID とオーナー名の誤りと独立に検証できるようにするため。
func ParseOwnerType(s string) (OwnerType, error) {
	ownerType := OwnerType(s)
	if ownerType != OwnerTypeOrganization && ownerType != OwnerTypeUser {
		return "", fmt.Errorf("%w: owner type must be %s or %s", ErrInvalidAllowedOwner, OwnerTypeOrganization, OwnerTypeUser)
	}
	return ownerType, nil
}
