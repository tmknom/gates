package authz

import (
	"slices"

	"github.com/tmknom/gates/internal/caller"
	"github.com/tmknom/gates/internal/githubid"
	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/protected"
	"github.com/tmknom/gates/internal/repository"
	"github.com/tmknom/gates/internal/request"
)

// PolicyReadGrant は、ポリシー読み取りトークンの発行 API の呼び出しの内容を定めるポリシー読み取り許可である。
// Permissions をフィールドに持たず固定の値を返すため、呼び出し側は権限を変えられない。
// フィールドを非公開にし、ゼロ値以外は GrantPolicyRead でしか作れないようにする。
type PolicyReadGrant struct {
	installationID githubid.InstallationID
	repositories   []repository.FullName
}

// GrantPolicyRead は、発行上限と保護対象の条件を含む認可条件と Installation の有効性（RULE-010、RULE-011）を満たすときだけ PolicyReadGrant を確定する。
// 要求の Permissions は認可の判定にだけ使い、ポリシー読み取り許可には写さない。
func GrantPolicyRead(c caller.Caller, req request.TokenRequest, allowedOwner AllowedOwner, issuablePermissions permission.IssuablePermissions, protectedRepositories protected.Repositories, inst Installation) (PolicyReadGrant, error) {
	if err := Authorize(c, req, allowedOwner, issuablePermissions, protectedRepositories); err != nil {
		return PolicyReadGrant{}, err
	}
	if err := inst.verifyUsableBy(allowedOwner); err != nil {
		return PolicyReadGrant{}, err
	}
	return PolicyReadGrant{installationID: inst.id, repositories: req.Repositories()}, nil
}

// IsZero は、GrantPolicyRead を経ていないゼロ値かを返す。
// GrantPolicyRead は NewInstallation を経た Installation からだけ作るため、Installation ID がゼロ値の値はゼロ値に限る。
func (g PolicyReadGrant) IsZero() bool {
	return g.installationID.IsZero()
}

// InstallationID は、発行に使う Installation の ID を返す。
func (g PolicyReadGrant) InstallationID() githubid.InstallationID {
	return g.installationID
}

// Repositories は、リポジトリポリシーを読む対象の完全名を要求の順序と表記のまま返す。
// 呼び出し元が並べ替えや書き換えをしても、確定したポリシー読み取り許可が変わらないように写す。
func (g PolicyReadGrant) Repositories() []repository.FullName {
	return slices.Clone(g.repositories)
}

// Permissions は、ポリシー読み取りトークンに要求する権限を返す。
func (g PolicyReadGrant) Permissions() permission.Permissions {
	return permission.PolicyRead()
}
