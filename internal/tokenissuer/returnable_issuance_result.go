package tokenissuer

import (
	"slices"
	"time"

	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/permission"
)

// ReturnableIssuanceResult は、発行許可との照合を通った、利用者に返せる発行結果である。
// フィールドを非公開にし、ゼロ値以外は照合（verify）でしか作れないようにする。
type ReturnableIssuanceResult struct {
	token        github.InstallationAccessToken
	expiresAt    time.Time
	permissions  permission.Permissions
	repositories []github.EffectiveRepository
}

// Token は、Installation Access Token を返す。
func (r ReturnableIssuanceResult) Token() github.InstallationAccessToken {
	return r.token
}

// ExpiresAt は、GitHub が定めた有効期限を返す。
func (r ReturnableIssuanceResult) ExpiresAt() time.Time {
	return r.expiresAt
}

// EffectivePermissions は、実効権限を返す。
func (r ReturnableIssuanceResult) EffectivePermissions() permission.Permissions {
	return r.permissions
}

// EffectiveRepositories は、実効対象を発行応答の順序と表記のまま返す。
// 呼び出し元が並べ替えや書き換えをしても、照合を通った値が変わらないように写す。
func (r ReturnableIssuanceResult) EffectiveRepositories() []github.EffectiveRepository {
	return slices.Clone(r.repositories)
}
