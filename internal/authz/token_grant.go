package authz

import (
	"slices"

	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/repository"
)

// TokenGrant は、認可が成立した結果として、GitHub API への発行要求の内容を定める発行許可である。
// フィールドを非公開にし、ゼロ値以外は Grant でしか作れないようにする。
type TokenGrant struct {
	installationID int64
	repositories   []repository.FullName
	permissions    permission.Permissions
}

// InstallationID は、発行に使う Installation の ID を返す。
func (g TokenGrant) InstallationID() int64 {
	return g.installationID
}

// Repositories は、発行の対象の完全名を要求の順序と表記のまま返す。
// 呼び出し元が並べ替えや書き換えをしても、確定した発行許可が変わらないように写す。
func (g TokenGrant) Repositories() []repository.FullName {
	return slices.Clone(g.repositories)
}

// Permissions は、発行する権限を返す。
func (g TokenGrant) Permissions() permission.Permissions {
	return g.permissions
}
