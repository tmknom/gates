package handler

import (
	"github.com/tmknom/gates/internal/audit"
	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/caller"
	"github.com/tmknom/gates/internal/repositorypolicy"
	"github.com/tmknom/gates/internal/request"
)

// issuanceAuthorization は、一回の発行処理の Caller と発行要求に、Handler の固定設定を添えて許可を確定する Authorization である。
// Token Issuer が Caller と発行要求を知らずに、取得した Installation から許可を確定できるようにする。
type issuanceAuthorization struct {
	handler      *Handler
	caller       caller.Caller
	tokenRequest request.TokenRequest
	record       *audit.Record
}

// GrantPolicyRead は、ポリシー読み取り許可を確定する。
func (a issuanceAuthorization) GrantPolicyRead(installation authz.Installation) (authz.PolicyReadGrant, error) {
	// 拒否しても、Installation の解決には成功しているため、判定の前に詰める
	a.record.SetInstallation(installation)
	h := a.handler
	return authz.GrantPolicyRead(a.caller, a.tokenRequest, h.allowedOwner, h.issuablePermissions, h.protectedRepositories, h.enforcement, installation)
}

// GrantWithRepositoryPolicies は、取得したリポジトリポリシーで発行許可を確定する。
func (a issuanceAuthorization) GrantWithRepositoryPolicies(installation authz.Installation, fetchResults []repositorypolicy.FetchResult) (authz.TokenGrant, error) {
	// どの確定でも詰め、Token Issuer が呼ぶ順序によらず解決した Installation を残す
	a.record.SetInstallation(installation)
	h := a.handler
	return authz.GrantWithRepositoryPolicies(a.caller, a.tokenRequest, h.allowedOwner, h.issuablePermissions, h.protectedRepositories, h.enforcement, installation, fetchResults)
}

// Grant は、リポジトリポリシーを使わずに発行許可を確定する。
func (a issuanceAuthorization) Grant(installation authz.Installation) (authz.TokenGrant, error) {
	// 拒否しても、Installation の解決には成功しているため、判定の前に詰める
	a.record.SetInstallation(installation)
	h := a.handler
	return authz.Grant(a.caller, a.tokenRequest, h.allowedOwner, h.issuablePermissions, h.protectedRepositories, h.enforcement, installation)
}
