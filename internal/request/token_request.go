package request

import (
	"slices"

	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/repository"
)

// TokenRequest は、Payload から GATES 用 OIDC JWT を除いた発行要求である。
// フィールドを非公開にし、ゼロ値以外は Validate でしか作れないようにする。
type TokenRequest struct {
	repositories []repository.FullName
	permissions  permission.AppPermissions
}

// Repositories は、要求された完全名を要求の順序と表記のまま返す。
// 呼び出し元が並べ替えや書き換えをしても、検証済みの要求と監査記録に残す内容が変わらないように写す。
func (r TokenRequest) Repositories() []repository.FullName {
	return slices.Clone(r.repositories)
}

// Permissions は、要求された App Permissions を返す。
func (r TokenRequest) Permissions() permission.AppPermissions {
	return r.permissions
}
