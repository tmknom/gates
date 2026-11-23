module gates/authorization

// GATES authorization model, Current Scope.
// Source of truth: FR-AUTHZ-001..012, FR-ISSUANCE-002, FR-ISSUANCE-003, FR-ISSUANCE-004,
// FR-ISSUANCE-005, FR-ISSUANCE-007, FR-OPS-001,
// RULE-010, RULE-011, RULE-015, RULE-016, INV-002, INV-011, INV-012, INV-015, INV-016,
// INV-017, INV-019, INV-020;
// docs/design/security.md 3.1..3.7; docs/design/domain.md Permission Levels,
// Protected Repositories and Repository Policies.
// This is a finite structural abstraction, NOT the complete GATES implementation.
// Cryptographic verification, HTTP outcomes, time, revocation, retry and
// configuration changes are outside this model.
// All Caller atoms represent already-verified GitHub OIDC identities.
// Names follow docs/design/domain.md. An Owner atom stands for one Owner ID and
// a Repository atom for one Repository ID, so field names omit the "ID" suffix.

sig Owner {}
sig OwnerName {}
// A Repository Name is the name of a repository without its owner name.
// Name atoms stand for names compared without letter case.
sig RepositoryName {}
sig FullName { ownerName: one OwnerName, repositoryName: one RepositoryName }
// `fullName` is the current name on GitHub. It is a field of one instance, so
// the protected repositories and the allowed callers can hold an ID with a
// name other than its current one (a rename) and a name whose current
// repository has another ID (a re-creation or a mistyped ID). Two repositories
// can share a full name: one of them stands for a repository deleted and
// re-created under the same name between two reads.
// `policy` is the Repository Policy on its default branch when GATES reads it.
sig Repository { owner: one Owner, fullName: one FullName, policy: one PolicyState }

// What reading the Repository Policy of a repository yields when the read
// itself succeeds: no file, an invalid document (not a file, too large, a
// format or content error), or a valid Repository Policy (INV-017).
// The commands give PolicyState a scope of 4, so that two targets of one
// request can have different valid Repository Policies.
abstract sig PolicyState {}
one sig NoPolicy, InvalidPolicy extends PolicyState {}
sig ValidPolicy extends PolicyState { allowedCallers: set AllowedCaller }

// Repository Policy Enforcement, fixed per deployment.
abstract sig Enforcement {}
one sig Enforced, NotEnforced extends Enforcement {}

// A Full Name is a value: one atom per pair of owner name and repository name.
fact FullNamesAreValues {
  all disj x, y: FullName | x.ownerName != y.ownerName or x.repositoryName != y.repositoryName
}

// The categories GitHub uses for GitHub App permissions. Only Repository
// permissions are limited by the repository-scoped authorization and
// verification; the others reach beyond the requested repositories.
abstract sig PermissionCategory {}
one sig RepositoryPermissions, OrganizationPermissions,
        AccountPermissions, EnterprisePermissions extends PermissionCategory {}

// Two levels stand for the levels a permission name can have. `admin` of
// `repository_projects` is not modeled: Level Inclusion compares one requested
// level with one level of the limit, and a third level adds no case that the
// rule treats differently.
abstract sig Level {}
one sig Read, Write extends Level {}

// A permission name and the levels GitHub defines for it.
abstract sig PermissionName {
  category: one PermissionCategory,
  definedLevels: some Level
}
// Names GATES enumerates in the allowed permissions.
sig ListedPermissionName extends PermissionName {}
one sig Metadata extends ListedPermissionName {}
// `single_file`, the only permission of the Policy Read Token.
one sig SingleFile extends ListedPermissionName {}
// Names GATES does not enumerate: Organization, Account and Enterprise
// permissions, Repository permissions whose category is not confirmed, and
// names GitHub adds later. Their category is left free.
sig UnlistedPermissionName extends PermissionName {}

// A Permission Entry is a value: one atom per pair of name and level, and every
// pair exists, including levels GitHub does not define for the name.
sig PermissionEntry { name: one PermissionName, level: one Level }

fact PermissionEntriesAreValues {
  all disj x, y: PermissionEntry | x.name != y.name or x.level != y.level
  all n: PermissionName, l: Level | some e: PermissionEntry | e.name = n and e.level = l
}

fact MetadataLevels { Metadata.definedLevels = Read + Write }
fact SingleFileLevels { Read in SingleFile.definedLevels }

// GATES adds a name to the allowed permissions only after confirming in the
// GitHub App settings that it is a Repository permission (security.md 3.3).
// This is an assumption on the enumerated content, not a runtime check.
fact ListedNamesAreRepositoryPermissions {
  ListedPermissionName.category = RepositoryPermissions
}

// The enumerated, fixed allowed permissions: listed names at their defined
// levels. They do not depend on the Caller or the target repositories.
// A listed name can hold `write` without `read`.
fun allowedPermissions: set PermissionEntry {
  { e: PermissionEntry | e.name in ListedPermissionName and e.level in e.name.definedLevels }
}
fun impliedPermissions: set PermissionEntry {
  { e: PermissionEntry | e.name = Metadata and e.level = Read }
}
// The Permissions of the Policy Read Token, fixed by GATES: `single_file:read`.
fun policyReadPermissions: set PermissionEntry {
  { e: PermissionEntry | e.name = SingleFile and e.level = Read }
}

// Permissions are keyed by permission name, so a name has one level.
pred oneLevelPerName[s: set PermissionEntry] {
  all disj x, y: s | x.name != y.name
}

// Level Order: within one permission name, `write` is higher than `read`.
// Levels of different names are never compared.
fun levelsAtOrBelow[l: Level]: set Level {
  (l = Write) => (Read + Write) else Read
}
// The entries of the same name at the same or a lower level than an entry of
// `held`. It also stands for GitHub's behavior that an Installation granted
// `write` can issue `read` of the same name.
fun coveredBy[held: set PermissionEntry]: set PermissionEntry {
  { e: PermissionEntry | some h: held | h.name = e.name and e.level in levelsAtOrBelow[h.level] }
}

// Level Inclusion: a limit contains `p:l` when `p:l` is in the allowed
// permissions and the limit holds `p` at `l` or a higher level. Two variants
// drop the condition on the allowed permissions.
// trace:formalizes FR-AUTHZ-008 FR-AUTHZ-009
fun levelInclusion[limit: set PermissionEntry]: set PermissionEntry {
  (ModelConfiguration.policy in WithoutInclusionAllowedCondition + WithoutBothAllowedPermissionGuards)
    => coveredBy[limit]
    else (allowedPermissions & coveredBy[limit])
}

abstract sig RepositorySelection {}
one sig Selected, AllRepositories extends RepositorySelection {}

abstract sig Answer {}
one sig Yes, No extends Answer {}

// `repositoryName` is the name paired with the Repository ID in `sub`. GitHub
// does not guarantee that it equals the current name of `repository`, so the
// two are left independent.
sig Caller { owner: one Owner, repository: one Repository, repositoryName: one RepositoryName }

sig Installation {
  installationOwner: one Owner,
  suspended: one Answer,
  coveredRepositories: set Repository,
  installedPermissions: set PermissionEntry
}

// An Allowed Caller: the Repository ID and the Repository Name of a Caller,
// and its Permission Limit. Neither needs to match the current state of
// GitHub, so a mistyped ID or name, a rename, and a re-creation can appear.
sig AllowedCaller {
  repository: one Repository,
  repositoryName: one RepositoryName,
  permissionLimit: some PermissionEntry
}

// A Protected Repository: its Repository ID, its Repository Name, and its
// Allowed Callers, which can be empty. The ID and the name need not match
// the current state of GitHub.
sig ProtectedRepository {
  repository: one Repository,
  repositoryName: one RepositoryName,
  allowedCallers: set AllowedCaller
}

// The issuable permissions are fixed configuration chosen by the GATES
// operator for one deployment. Like the allowed permissions they are a set of
// Permission Entries, may hold several levels of one name, and do not depend
// on the Caller or the target repositories. The protected repositories are
// fixed configuration too, and can be empty. Repository Policy Enforcement is
// one of two values (INV-018); both are explored.
one sig Config {
  allowedOwner: one Owner,
  allowedOwnerName: one OwnerName,
  issuablePermissions: set PermissionEntry,
  protectedRepositories: set ProtectedRepository,
  repositoryPolicyEnforcement: one Enforcement
}

pred enforced { Config.repositoryPolicyEnforcement = Enforced }

sig TokenRequest {
  repositories: some FullName,
  permissions: some PermissionEntry
}

// `caller` and `request` trace where the grant came from; they are not part of
// its content, which is the installation, repositories and permissions.
// `policyToken` is the Verified Policy Read Token whose reads the grant relied
// on; it exists exactly when Repository Policies are enforced.
sig TokenGrant {
  caller: one Caller,
  request: one TokenRequest,
  installation: one Installation,
  repositories: some FullName,
  permissions: some PermissionEntry,
  policyToken: lone VerifiedPolicyReadToken
}

// The Policy Read Grant: the result of every condition of the Token Grant
// except RULE-016. Its Permissions are fixed by GATES, not taken from the request.
sig PolicyReadGrant {
  caller: one Caller,
  request: one TokenRequest,
  installation: one Installation,
  repositories: some FullName,
  permissions: some PermissionEntry
}

// The GitHub response to one issuance of a Policy Read Token. It has the
// shape of an Issuance Result; `singleFilePathIsFixed` stands for the paths
// the token can read being exactly the Repository Policy Path.
sig PolicyReadResult {
  policyReadGrant: one PolicyReadGrant,
  nonemptyToken: one Answer,
  futureExpiry: one Answer,
  effectiveRepositories: set EffectiveRepository,
  effectivePermissions: set PermissionEntry,
  repositorySelection: one RepositorySelection,
  singleFilePathIsFixed: one Answer
}

sig VerifiedPolicyReadToken { readResult: one PolicyReadResult }

// One successful read of a Repository Policy with a Verified Policy Read
// Token, by full name. A read that failed has no atom, so a missing atom
// stands for the transient failure. `readRepository` is the repository the
// name pointed to when GATES read it. The response carries no Repository ID,
// so nothing ties it to the token's effective repository of that name; that
// tie is the assumption `fetchReadsTokenRepository`.
sig PolicyFetch {
  token: one VerifiedPolicyReadToken,
  fullName: one FullName,
  readRepository: one Repository
}

// One invocation of the GitHub issuance API.
sig IssueCall {
  grant: one TokenGrant,
  repositories: some FullName,
  permissions: some PermissionEntry
}

// One element of the result's repository list. Separate atoms can refer to
// the same repository, so duplicates are representable. Its full name is the
// current name of the repository.
sig EffectiveRepository { repository: one Repository }

sig IssuanceResult {
  call: one IssueCall,
  nonemptyToken: one Answer,
  futureExpiry: one Answer,
  effectiveRepositories: set EffectiveRepository,
  effectivePermissions: set PermissionEntry,
  repositorySelection: one RepositorySelection
}

sig ReturnableIssuanceResult { result: one IssuanceResult }

// The baseline policy is used by the original checks and witnesses.
// The other policies deliberately disable a check: the allowed permissions
// check of authorization, the condition on the allowed permissions within
// Level Inclusion, both of them, the invariant that the issuable permissions
// are part of the allowed permissions (INV-015, checked when the configuration
// is read), that invariant together with the allowed permissions check, the
// issuable permissions check of authorization, one return-time check, one
// check on the protected repositories, or one check on the Repository Policies
// and the Policy Read Token.
// They are counterfactual verification models, not GATES configurations.
abstract sig VerificationPolicy {}
one sig Baseline, WithoutAllowedPermissionCheck, WithoutInclusionAllowedCondition,
        WithoutBothAllowedPermissionGuards, WithoutIssuableSubsetInvariant,
        WithoutAllowedPermissionCheckAndIssuableSubsetInvariant,
        WithoutIssuablePermissionCheck,
        WithoutOwnerCheck, WithoutNameCheck,
        WithoutExcessPermissionCheck, WithoutMissingPermissionCheck,
        WithoutCountCheck,
        WithoutProtectedNameCheck, WithoutAllowedCallerNameCheck,
        WithoutPermissionLimitCheck, WithoutProtectedIdCheck,
        WithoutAllowedCallerIdCheck, WithoutRepositoryPolicyCheck,
        WithoutPolicyAllTargets, WithoutPolicyPermissionLimitCheck,
        WithoutEnforcementSwitch, WithoutProtectedCheckWhenEnforced,
        WithoutPolicyRepositoryIdCheck, WithoutPolicyReadPermissionCheck,
        WithoutPolicyReadPathCheck,
        WithoutPolicyReadAuthorization extends VerificationPolicy {}
one sig ModelConfiguration { policy: one VerificationPolicy }

pred baseline { ModelConfiguration.policy = Baseline }

// A single GitHub issuance API invocation can yield at most one modeled
// result, and one result can become at most one returnable issuance result.
fact ResultCardinality {
  all c: IssueCall | lone c.~call
  all r: IssuanceResult | lone r.~result
  all r: PolicyReadResult | lone r.~readResult
}

// A token reads each requested name at most once, and only requested names.
fact PolicyFetchesFollowTheGrant {
  all f: PolicyFetch | f.fullName in f.token.readResult.policyReadGrant.repositories
  all t: VerifiedPolicyReadToken, n: FullName | lone fetchesOf[t, n]
}

fun fetchesOf[t: VerifiedPolicyReadToken, n: FullName]: set PolicyFetch {
  { f: PolicyFetch | f.token = t and f.fullName = n }
}

// Every Permissions value is keyed by permission name: the request after
// format validation (INV-002), the Installation's granted permissions, and the
// effective permissions in the GitHub response.
fact PermissionsHaveOneLevelPerName {
  all r: TokenRequest | oneLevelPerName[r.permissions]
  all i: Installation | oneLevelPerName[i.installedPermissions]
  all r: IssuanceResult | oneLevelPerName[r.effectivePermissions]
}

// INV-015: the configuration is read only when the issuable permissions are
// not empty and are part of the allowed permissions. Otherwise initialization
// fails and no issuance process starts, so no such state is modeled.
fact IssuablePermissionsInvariant {
  some Config.issuablePermissions
  (ModelConfiguration.policy not in WithoutIssuableSubsetInvariant + WithoutAllowedPermissionCheckAndIssuableSubsetInvariant) implies
    Config.issuablePermissions in allowedPermissions
}

// INV-016: the protected repositories share no Repository ID and no
// Repository Name, and neither do the allowed callers of one protected
// repository. A Permission Limit is a non-empty Permissions within the
// allowed permissions, and need not be within the issuable permissions.
// Allowed callers of different protected repositories may share an ID with
// different names: that combination is not excluded.
// trace:formalizes INV-016
fact ProtectedRepositoriesInvariant {
  all disj x, y: Config.protectedRepositories |
    x.repository != y.repository and x.repositoryName != y.repositoryName
  all p: Config.protectedRepositories | all disj x, y: p.allowedCallers |
    x.repository != y.repository and x.repositoryName != y.repositoryName
  all a: Config.protectedRepositories.allowedCallers |
    oneLevelPerName[a.permissionLimit] and a.permissionLimit in allowedPermissions
}

// INV-017: the allowed callers of a Repository Policy share no Repository ID
// and no Repository Name, and each Permission Limit is a non-empty Permissions
// within the allowed permissions, as for INV-016. A document breaking this is
// not a ValidPolicy but an InvalidPolicy. A ValidPolicy can have no allowed
// callers. An Allowed Caller atom can appear in both a Repository Policy and
// the protected repositories.
// trace:formalizes INV-017
fact RepositoryPoliciesInvariant {
  all p: ValidPolicy | all disj x, y: p.allowedCallers |
    x.repository != y.repository and x.repositoryName != y.repositoryName
  all a: ValidPolicy.allowedCallers |
    oneLevelPerName[a.permissionLimit] and a.permissionLimit in allowedPermissions
}

// Identity consistency comes from successful OIDC verification; it is not
// checked again by authorization. The caller's repository is an ACL key for
// the protected repositories and, under Repository Policy Enforcement, for
// every target.
fact VerifiedCallerRepository {
  all c: Caller | c.repository.owner = c.owner
}

// The installation is derived from the fixed GitHub App and the allowed owner,
// not configured. Every Installation atom belongs to that one GitHub App, and
// at most one exists per owner: an *external GitHub behavior assumption*
// inferred in docs/design/domain.md 3.1, not a guarantee stated by GitHub.
fact OneInstallationPerOwner {
  all disj x, y: Installation | x.installationOwner != y.installationOwner
}

// The protected repositories that a request targets: those whose Repository
// Name equals the repository part of a requested full name. Before issuance
// only names are known, so the Repository ID is not compared here.
fun targetedProtectedRepositories[r: TokenRequest]: set ProtectedRepository {
  { p: Config.protectedRepositories | p.repositoryName in r.repositories.repositoryName }
}

// A Caller is an Allowed Caller when both its Repository ID and its Repository
// Name equal those of the Allowed Caller. The protected repositories and the
// Repository Policies share this condition.
pred isAllowedCaller[c: Caller, a: AllowedCaller] {
  (ModelConfiguration.policy != WithoutAllowedCallerIdCheck) implies
    a.repository = c.repository
  (ModelConfiguration.policy != WithoutAllowedCallerNameCheck) implies
    a.repositoryName = c.repositoryName
}

// RULE-015: every targeted protected repository has an Allowed Caller that the
// Caller is, and whose Permission Limit includes every requested entry by
// Level Inclusion. A protected repository without Allowed Callers fails for
// every Caller; a request targeting none is not restricted.
// trace:formalizes RULE-015 FR-AUTHZ-009 FR-AUTHZ-010
pred protectedRepositoriesAuthorized[c: Caller, r: TokenRequest] {
  (ModelConfiguration.policy != WithoutProtectedNameCheck) implies
    (all p: targetedProtectedRepositories[r] | some a: p.allowedCallers |
      isAllowedCaller[c, a] and
      ((ModelConfiguration.policy != WithoutPermissionLimitCheck) implies
        r.permissions in levelInclusion[a.permissionLimit]))
}

// RULE-010 and RULE-011: the owner, the allowed permissions, the issuable
// permissions by Level Inclusion, and the validity of the installation. The
// owner ID and owner name are fixed configuration; the owner name is modeled
// separately from the immutable owner ID. A request with any entry outside the
// allowed permissions or the issuable permissions is rejected as a whole,
// never narrowed or raised.
// trace:formalizes RULE-010 RULE-011
pred ownerAndPermissionsGrantable[c: Caller, r: TokenRequest, i: Installation] {
  c.owner = Config.allowedOwner
  all n: r.repositories | n.ownerName = Config.allowedOwnerName
  (ModelConfiguration.policy not in WithoutAllowedPermissionCheck + WithoutBothAllowedPermissionGuards +
     WithoutAllowedPermissionCheckAndIssuableSubsetInvariant) implies
    r.permissions in allowedPermissions
  (ModelConfiguration.policy != WithoutIssuablePermissionCheck) implies
    r.permissions in levelInclusion[Config.issuablePermissions]
  i.installationOwner = Config.allowedOwner
  i.suspended = No
}

// The conditions without Repository Policies: those of the Grant operation of
// a deployment that does not enforce them, and those of the Policy Read Grant.
pred grantable[c: Caller, r: TokenRequest, i: Installation] {
  ownerAndPermissionsGrantable[c, r, i]
  protectedRepositoriesAuthorized[c, r]
}

// The Repository Policy read under the name `n` allows the Caller and every
// requested entry. A missing read (a transient failure), NoPolicy,
// InvalidPolicy and a ValidPolicy without allowed callers all fail, because
// only a ValidPolicy has allowed callers.
pred targetAllowedByPolicy[c: Caller, r: TokenRequest, t: VerifiedPolicyReadToken, n: FullName] {
  some f: fetchesOf[t, n], a: f.readRepository.policy.allowedCallers |
    isAllowedCaller[c, a] and
    ((ModelConfiguration.policy != WithoutPolicyPermissionLimitCheck) implies
      r.permissions in levelInclusion[a.permissionLimit])
}

// RULE-016: every requested name, protected or not and the Caller's own
// repository included, is allowed by the Repository Policy read with the
// token. One name that is not allowed rejects the whole request.
// trace:formalizes RULE-016 FR-AUTHZ-011
pred repositoryPolicyAuthorized[c: Caller, r: TokenRequest, t: VerifiedPolicyReadToken] {
  ModelConfiguration.policy = WithoutRepositoryPolicyCheck or
  (ModelConfiguration.policy = WithoutPolicyAllTargets and
    (some n: r.repositories | targetAllowedByPolicy[c, r, t, n])) or
  (ModelConfiguration.policy not in WithoutRepositoryPolicyCheck + WithoutPolicyAllTargets and
    (all n: r.repositories | targetAllowedByPolicy[c, r, t, n]))
}

// Whether the deployment reads Repository Policies and judges RULE-016: with
// enforcement only, except in the variant that ignores the setting.
pred policiesApplied {
  enforced or ModelConfiguration.policy = WithoutEnforcementSwitch
}

// The conditions of the Grant operation (INV-011): `t` is the Verified Policy
// Read Token of the grant, empty when Repository Policies are not enforced.
// With enforcement, RULE-016 is added to the other conditions, and the token
// must come from a Policy Read Grant for the same Caller, request and
// Installation.
pred tokenGrantConditions[c: Caller, r: TokenRequest, i: Installation, t: set VerifiedPolicyReadToken] {
  ownerAndPermissionsGrantable[c, r, i]
  (ModelConfiguration.policy = WithoutProtectedCheckWhenEnforced and enforced) or
    protectedRepositoriesAuthorized[c, r]
  policiesApplied implies
    (one t and
     t.readResult.policyReadGrant.caller = c and
     t.readResult.policyReadGrant.request = r and
     t.readResult.policyReadGrant.installation = i and
     repositoryPolicyAuthorized[c, r, t])
}

// Represents the checks before the GitHub issuance API call. In particular,
// actual repo ownership / installation coverage and the Installation's granted
// permissions are NOT preconditions here. The grant keeps the requested
// levels, never the higher levels of a limit.
// trace:formalizes INV-011
fact GrantsRespectAuthorization {
  all g: TokenGrant |
    tokenGrantConditions[g.caller, g.request, g.installation, g.policyToken] and
    (not policiesApplied implies no g.policyToken) and
    g.repositories = g.request.repositories and
    g.permissions = g.request.permissions
}

// INV-020: a Policy Read Grant exists only with enforcement and only for a
// request meeting every condition except RULE-016. It names the requested
// repositories and only `single_file:read`, whatever the request asks.
// trace:formalizes INV-020
fact PolicyReadGrantsRespectAuthorization {
  all g: PolicyReadGrant |
    policiesApplied and
    ((ModelConfiguration.policy != WithoutPolicyReadAuthorization) implies
      (ownerAndPermissionsGrantable[g.caller, g.request, g.installation] and
       (ModelConfiguration.policy = WithoutProtectedCheckWhenEnforced or
        protectedRepositoriesAuthorized[g.caller, g.request]))) and
    g.repositories = g.request.repositories and
    g.permissions = policyReadPermissions
}

// The API call always specifies exact requested names and permissions;
// it never inherits all repositories or all permissions by omission.
fact ExplicitIssueCalls {
  all c: IssueCall |
    c.repositories = c.grant.repositories and
    c.permissions = c.grant.permissions
}

// The return-time checks against the grant.
// Permissions are compared as pairs of name and level, so an entry at a lower
// or higher level than granted fails one of the two permission checks.
pred resultMatchesGrant[r: IssuanceResult] {
  r.nonemptyToken = Yes
  r.futureExpiry = Yes
  r.repositorySelection = Selected

  // Equal count and equal name set leave no room for a repeated name or ID.
  (ModelConfiguration.policy != WithoutCountCheck) implies
    #(r.effectiveRepositories) = #(r.call.grant.repositories)
  (ModelConfiguration.policy != WithoutNameCheck) implies
    r.effectiveRepositories.repository.fullName = r.call.grant.repositories
  (ModelConfiguration.policy != WithoutOwnerCheck) implies
    (all x: r.effectiveRepositories.repository | x.owner = Config.allowedOwner)
  (ModelConfiguration.policy != WithoutMissingPermissionCheck) implies
    r.call.grant.permissions in r.effectivePermissions
  (ModelConfiguration.policy != WithoutExcessPermissionCheck) implies
    r.effectivePermissions in (r.call.grant.permissions + impliedPermissions)
}

// For every effective repository and every protected repository, the names
// match exactly when the IDs match. Every pair is checked, not only the
// protected repository the name targets, so a renamed protected repository
// requested by its new name fails too.
// trace:formalizes INV-012 FR-AUTHZ-010
pred protectedRepositoriesMatchResult[r: IssuanceResult] {
  protectedRepositoriesMatch[r.effectiveRepositories.repository]
}

pred protectedRepositoriesMatch[xs: set Repository] {
  all x: xs, p: Config.protectedRepositories |
    x.fullName.repositoryName = p.repositoryName iff x = p.repository
}

// Each effective repository has the Repository ID of the repository whose
// Repository Policy was read under the same full name, as the Verified Policy
// Read Token of the grant shows it. Without enforcement there is no token and
// the condition holds.
// trace:formalizes INV-012 FR-AUTHZ-012
pred policyRepositoriesMatchResult[r: IssuanceResult] {
  all x: r.effectiveRepositories.repository,
      y: r.call.grant.policyToken.readResult.effectiveRepositories.repository |
    x.fullName = y.fullName implies x = y
}

// Represents return-time checks (INV-012), NOT a guarantee about the actual
// GitHub token's authorization outside a trustworthy GitHub response.
pred verifiedResult[r: IssuanceResult] {
  resultMatchesGrant[r]
  (ModelConfiguration.policy != WithoutProtectedIdCheck) implies
    protectedRepositoriesMatchResult[r]
  (ModelConfiguration.policy != WithoutPolicyRepositoryIdCheck) implies
    policyRepositoriesMatchResult[r]
}

// INV-019: the checks on the response to a Policy Read Token issuance. They
// are those of INV-012 against the Policy Read Grant, the Permissions are
// exactly `single_file:read` with the optional implied ones, and the token
// reads only the Repository Policy Path.
// trace:formalizes INV-019 FR-ISSUANCE-004 FR-ISSUANCE-007
pred policyReadResultVerified[r: PolicyReadResult] {
  r.nonemptyToken = Yes
  r.futureExpiry = Yes
  r.repositorySelection = Selected
  (ModelConfiguration.policy != WithoutPolicyReadPathCheck) implies
    r.singleFilePathIsFixed = Yes
  #(r.effectiveRepositories) = #(r.policyReadGrant.repositories)
  r.effectiveRepositories.repository.fullName = r.policyReadGrant.repositories
  all x: r.effectiveRepositories.repository | x.owner = Config.allowedOwner
  (ModelConfiguration.policy != WithoutPolicyReadPermissionCheck) implies
    (r.policyReadGrant.permissions in r.effectivePermissions and
     r.effectivePermissions in (r.policyReadGrant.permissions + impliedPermissions))
  protectedRepositoriesMatch[r.effectiveRepositories.repository]
}

fact OnlyVerifiedPolicyReadResultsBecomeTokens {
  all t: VerifiedPolicyReadToken | policyReadResultVerified[t.readResult]
}

// No race on the name during the read: the read reached the token's effective
// repository of that name. GATES cannot check it, because the response of
// the read carries no Repository ID (security.md 3.7 accepts the risk).
pred fetchReadsTokenRepository[f: PolicyFetch] {
  some x: f.token.readResult.effectiveRepositories.repository |
    x = f.readRepository and x.fullName = f.fullName
}

fact OnlyVerifiedResultsBecomeReturnable {
  all t: ReturnableIssuanceResult | verifiedResult[t.result]
}

// The following is an *external GitHub behavior assumption*, deliberately
// not a fact: GitHub must enforce installation coverage and app permissions.
// GATES does not prevalidate the requested names against coverage, nor the
// requested permissions against the Installation's granted permissions.
pred githubResultRespectsInstallation[r: IssuanceResult] {
  r.effectiveRepositories.repository in r.call.grant.installation.coveredRepositories
  r.effectivePermissions in (coveredBy[r.call.grant.installation.installedPermissions] + impliedPermissions)
}

fact PolicyReadResultPermissionsHaveOneLevelPerName {
  all r: PolicyReadResult | oneLevelPerName[r.effectivePermissions]
}

pred effectiveRepositoriesAreUnique[r: IssuanceResult] {
  all disj x, y: r.effectiveRepositories |
    x.repository != y.repository and x.repository.fullName != y.repository.fullName
}

assert NoForeignOwnerGrant {
  all g: TokenGrant | g.caller.owner = Config.allowedOwner
}
assert NoOtherInstallationGrant {
  lone TokenGrant.installation
  all g: TokenGrant |
    g.installation.installationOwner = Config.allowedOwner and g.installation.suspended = No
}
assert OnlyAllowedPermissionsGranted {
  baseline implies (all g: TokenGrant |
    g.permissions = g.request.permissions and g.permissions in allowedPermissions)
}
assert OnlyIssuablePermissionsGranted {
  baseline implies (all g: TokenGrant | g.permissions in levelInclusion[Config.issuablePermissions])
}
assert AllOrNothingGrant {
  all g: TokenGrant | g.repositories = g.request.repositories
}
assert AllIssueCallsExplicit {
  all c: IssueCall |
    c.repositories = c.grant.request.repositories and
    c.permissions = c.grant.request.permissions
}
assert NoOverbroadTokenReturned {
  baseline implies (all t: ReturnableIssuanceResult |
    t.result.effectivePermissions in (t.result.call.grant.request.permissions + impliedPermissions) and
    t.result.repositorySelection = Selected)
}
assert NoNarrowedTokenReturned {
  baseline implies (all t: ReturnableIssuanceResult |
    t.result.call.grant.request.permissions in t.result.effectivePermissions)
}
// When the GitHub App or the Installation holds allowed permissions beyond the
// request, a returned token carries none of them except the implied ones.
assert UnrequestedAllowedPermissionsNotReturned {
  baseline implies (all t: ReturnableIssuanceResult |
    no (allowedPermissions - t.result.call.grant.request.permissions - impliedPermissions)
      & t.result.effectivePermissions)
}
// The implied `metadata:read` is accepted at return even when the issuable
// permissions do not include it; every other returned entry is one that the
// issuable permissions include by Level Inclusion.
assert NoTokenBeyondIssuablePermissions {
  baseline implies (all t: ReturnableIssuanceResult |
    t.result.effectivePermissions in (levelInclusion[Config.issuablePermissions] + impliedPermissions))
}
// A returned entry has the requested level of its name, never the higher
// level of the issuable permissions or of a Permission Limit.
assert ReturnedLevelsEqualRequestedLevels {
  baseline implies (all t: ReturnableIssuanceResult |
    all e: t.result.effectivePermissions, q: t.result.call.grant.request.permissions |
      e.name = q.name implies e.level = q.level)
}
// A `write` in the issuable permissions does not admit `read` of the same
// name when the allowed permissions do not hold that `read`.
assert IssuableWriteAdmitsNoReadOutsideAllowedPermissions {
  baseline implies (no t: ReturnableIssuanceResult, e: t.result.effectivePermissions |
    e.level = Read and e not in allowedPermissions and
    (some b: Config.issuablePermissions | b.name = e.name and b.level = Write))
}
assert OnlyRepositoryPermissionsReturned {
  baseline implies (all t: ReturnableIssuanceResult |
    t.result.effectivePermissions.name.category in RepositoryPermissions)
}
assert ExactNamesReturned {
  baseline implies (all t: ReturnableIssuanceResult |
    t.result.effectiveRepositories.repository.fullName = t.result.call.grant.request.repositories and
    #(t.result.effectiveRepositories) = #(t.result.call.grant.request.repositories))
}
assert NoDuplicateEffectiveRepositoriesReturned {
  baseline implies (all t: ReturnableIssuanceResult | effectiveRepositoriesAreUnique[t.result])
}
assert ReturnedReposOwnedByAllowedOwner {
  baseline implies (all t: ReturnableIssuanceResult |
    all r: t.result.effectiveRepositories.repository | r.owner = Config.allowedOwner)
}
// Conditional on the external assumption: GATES does not prevalidate coverage,
// and its full-name check rejects a result that omits an uncovered name.
assert CoverageHoldsWhenGitHubEnforces {
  baseline implies (all t: ReturnableIssuanceResult |
    githubResultRespectsInstallation[t.result] implies
    (all n: t.result.call.grant.repositories |
      some r: t.result.call.grant.installation.coveredRepositories |
        r.fullName = n and r.owner = Config.allowedOwner))
}
// Conditional on the same assumption: GATES does not prevalidate the
// Installation's granted permissions, and its permission checks reject a
// result that omits or lowers a permission the Installation does not hold.
assert RequestedPermissionsHeldWhenGitHubEnforces {
  baseline implies (all t: ReturnableIssuanceResult |
    githubResultRespectsInstallation[t.result] implies
    t.result.call.grant.request.permissions in
      (coveredBy[t.result.call.grant.installation.installedPermissions] + impliedPermissions))
}
assert ReturnedResultsHaveValidTokenAndExpiry {
  all t: ReturnableIssuanceResult |
    t.result.nonemptyToken = Yes and t.result.futureExpiry = Yes
}

// A returned token for a repository whose ID is a protected repository's ID
// goes only to a Caller matching both the ID and the name of an Allowed Caller
// of that protected repository. A Caller matching only one of them, of every
// Allowed Caller, receives none.
pred protectedTokensOnlyToAllowedCallers {
  all t: ReturnableIssuanceResult, p: Config.protectedRepositories |
    p.repository in t.result.effectiveRepositories.repository implies
      (some a: p.allowedCallers |
        a.repository = t.result.call.grant.caller.repository and
        a.repositoryName = t.result.call.grant.caller.repositoryName)
}
assert ProtectedTokenNeedsIdAndNameOfAllowedCaller {
  baseline implies protectedTokensOnlyToAllowedCallers
}
// The returned permissions for a protected repository are those the
// Permission Limit of that Allowed Caller includes, and the implied ones.
pred protectedTokensWithinPermissionLimit {
  all t: ReturnableIssuanceResult, p: Config.protectedRepositories |
    p.repository in t.result.effectiveRepositories.repository implies
      (some a: p.allowedCallers |
        a.repository = t.result.call.grant.caller.repository and
        a.repositoryName = t.result.call.grant.caller.repositoryName and
        t.result.effectivePermissions in (levelInclusion[a.permissionLimit] + impliedPermissions))
}
assert ProtectedTokenWithinPermissionLimit {
  baseline implies protectedTokensWithinPermissionLimit
}
assert NoProtectedNameWithOtherIdReturned {
  baseline implies (no t: ReturnableIssuanceResult, p: Config.protectedRepositories,
                      x: t.result.effectiveRepositories.repository |
    x.fullName.repositoryName = p.repositoryName and x != p.repository)
}
// A request targeting no protected repository meets RULE-015 for every
// Caller, and with no protected repositories every result meets the added
// return-time check, so the other conditions decide as they did without them.
assert ProtectedRepositoriesAddNothingWhenNotTargeted {
  baseline implies {
    all c: Caller, r: TokenRequest |
      no targetedProtectedRepositories[r] implies protectedRepositoriesAuthorized[c, r]
    no Config.protectedRepositories implies
      (all r: IssuanceResult |
        verifiedResult[r] iff (resultMatchesGrant[r] and policyRepositoriesMatchResult[r]))
  }
}
// A Permission Limit outside the issuable or the allowed permissions does
// not widen what is returned.
assert ProtectedRepositoriesKeepPermissionBoundary {
  baseline implies (all t: ReturnableIssuanceResult |
    t.result.effectivePermissions in
      ((allowedPermissions & levelInclusion[Config.issuablePermissions]) + impliedPermissions))
}

// Repository Policies only add a condition: a grant made with enforcement
// meets every condition of a grant made without it.
pred grantsMeetConditionsWithoutPolicies {
  all g: TokenGrant | grantable[g.caller, g.request, g.installation]
}
assert RepositoryPoliciesOnlyAddConditions {
  baseline implies grantsMeetConditionsWithoutPolicies
}

// Without enforcement, no Repository Policy is read: no Policy Read Grant
// exists and no grant relies on a Policy Read Token. A grant then meets only
// the conditions without Repository Policies; `NotEnforcedTokenWithoutPolicy`
// shows a token returned for a repository without one.
pred decisionIgnoresPoliciesWhenNotEnforced {
  not enforced implies {
    no PolicyReadGrant
    all g: TokenGrant | no g.policyToken
  }
}
assert NotEnforcedIgnoresRepositoryPolicies {
  baseline implies decisionIgnoresPoliciesWhenNotEnforced
}

// Default deny: with enforcement, every requested name of a grant was read
// with the grant's token and yielded a valid Repository Policy that has an
// allowed caller.
pred grantsNeedValidPolicies {
  enforced implies (all g: TokenGrant, n: g.repositories |
    some f: fetchesOf[g.policyToken, n] |
      f.readRepository.policy in ValidPolicy and some f.readRepository.policy.allowedCallers)
}
assert DefaultDenyWithoutValidPolicy {
  baseline implies grantsNeedValidPolicies
}

// With enforcement, a grant for a request targeting a protected repository
// meets both RULE-015 and RULE-016: neither overrides the other.
pred protectedGrantsNeedListAndPolicy {
  enforced implies (all g: TokenGrant |
    some targetedProtectedRepositories[g.request] implies
      (protectedRepositoriesAuthorized[g.caller, g.request] and
       (all n: g.repositories | targetAllowedByPolicy[g.caller, g.request, g.policyToken, n])))
}
assert ProtectedRepositoriesNeedListAndPolicy {
  baseline implies protectedGrantsNeedListAndPolicy
}

// All or nothing: one requested name that its Repository Policy does not
// allow leaves no grant, whatever the other names allow.
pred grantsNeedEveryTargetAllowed {
  enforced implies (all g: TokenGrant, n: g.repositories |
    targetAllowedByPolicy[g.caller, g.request, g.policyToken, n])
}
assert EveryTargetAllowedByPolicy {
  baseline implies grantsNeedEveryTargetAllowed
}

// The granted Permissions lie within the allowed permissions, the issuable
// permissions, and the Permission Limit of an allowed caller the Caller
// matches in the Repository Policy of every requested name.
pred grantsWithinEveryPolicyLimit {
  all g: TokenGrant |
    g.permissions in (allowedPermissions & levelInclusion[Config.issuablePermissions]) and
    (enforced implies (all n: g.repositories |
      some f: fetchesOf[g.policyToken, n], a: f.readRepository.policy.allowedCallers |
        isAllowedCaller[g.caller, a] and g.permissions in levelInclusion[a.permissionLimit]))
}
assert GrantWithinEveryPermissionLimit {
  baseline implies grantsWithinEveryPolicyLimit
}

// A Caller matching only the Repository ID or only the Repository Name of
// every allowed caller of a Repository Policy gets no grant for that name.
pred policyGrantsNeedIdAndName {
  enforced implies (all g: TokenGrant, n: g.repositories |
    some f: fetchesOf[g.policyToken, n], a: f.readRepository.policy.allowedCallers |
      a.repository = g.caller.repository and a.repositoryName = g.caller.repositoryName)
}
assert PolicyAllowedCallerNeedsIdAndName {
  baseline implies policyGrantsNeedIdAndName
}

// FR-AUTHZ-012: every effective repository of a returned token is a
// repository whose Repository Policy GATES read, as identified by the
// Verified Policy Read Token of the grant.
pred returnedRepositoriesArePolicyRepositories {
  enforced implies (all t: ReturnableIssuanceResult |
    t.result.effectiveRepositories.repository in
      t.result.call.grant.policyToken.readResult.effectiveRepositories.repository)
}
assert ReturnedRepositoriesArePolicyRepositories {
  baseline implies returnedRepositoriesArePolicyRepositories
}

// Unless a name changed its repository during the read, every effective
// repository of a returned token has its own Repository Policy allowing the
// Caller and the requested Permissions.
pred returnedRepositoriesAllowedByOwnPolicy {
  enforced implies (all t: ReturnableIssuanceResult |
    (all f: t.result.call.grant.policyToken.~token | fetchReadsTokenRepository[f]) implies
      (all x: t.result.effectiveRepositories.repository |
        some a: x.policy.allowedCallers |
          isAllowedCaller[t.result.call.grant.caller, a] and
          t.result.call.grant.request.permissions in levelInclusion[a.permissionLimit]))
}
assert ReturnedRepositoriesAllowedByOwnPolicy {
  baseline implies returnedRepositoriesAllowedByOwnPolicy
}

// Unless a name changed its repository during the read, the returned
// permissions for each effective repository are those the Permission Limit of
// an Allowed Caller in its own Repository Policy includes, and the implied
// ones.
pred policyTokensWithinPermissionLimit {
  enforced implies (all t: ReturnableIssuanceResult |
    (all f: t.result.call.grant.policyToken.~token | fetchReadsTokenRepository[f]) implies
      (all x: t.result.effectiveRepositories.repository |
        some a: x.policy.allowedCallers |
          isAllowedCaller[t.result.call.grant.caller, a] and
          t.result.effectivePermissions in (levelInclusion[a.permissionLimit] + impliedPermissions)))
}
assert PolicyTokenWithinPermissionLimit {
  baseline implies policyTokensWithinPermissionLimit
}

// The Policy Read Token holds only `single_file:read` and the implied
// permission, and reads only the Repository Policy Path.
pred policyReadTokensOnlyReadThePolicy {
  all t: VerifiedPolicyReadToken |
    t.readResult.effectivePermissions in (policyReadPermissions + impliedPermissions) and
    t.readResult.singleFilePathIsFixed = Yes
}
assert PolicyReadTokenOnlyReadsThePolicy {
  baseline implies policyReadTokensOnlyReadThePolicy
}

// A Policy Read Grant needs enforcement and every condition of the Token Grant
// except RULE-016. The checks on the issuance result are not among them. Its
// Permissions are `single_file:read` whatever the request asks.
pred policyReadGrantsMeetOtherConditions {
  all g: PolicyReadGrant |
    enforced and grantable[g.caller, g.request, g.installation] and
    g.permissions = policyReadPermissions
}
assert PolicyReadGrantMeetsOtherConditions {
  baseline implies policyReadGrantsMeetOtherConditions
}

// Mutation checks intentionally evaluate the same returned-token properties
// under weaker verification policies. Their results must be examined as
// counterexamples; they are not assertions about the baseline implementation.
pred returnsOnlyAllowedPermissions {
  all t: ReturnableIssuanceResult | t.result.effectivePermissions in allowedPermissions
}
// Either of the two guards alone keeps returned tokens within the allowed
// permissions; removing both breaks the property. INV-015 is not one of them.
assert WithoutAllowedPermissionCheckKeepsAllowedPermissions {
  ModelConfiguration.policy = WithoutAllowedPermissionCheck implies returnsOnlyAllowedPermissions
}
assert WithoutInclusionAllowedConditionKeepsAllowedPermissions {
  ModelConfiguration.policy = WithoutInclusionAllowedCondition implies returnsOnlyAllowedPermissions
}
assert WithoutBothAllowedPermissionGuardsPreservesAllowedPermissions {
  ModelConfiguration.policy = WithoutBothAllowedPermissionGuards implies returnsOnlyAllowedPermissions
}
assert WithoutIssuableSubsetInvariantKeepsAllowedPermissions {
  ModelConfiguration.policy = WithoutIssuableSubsetInvariant implies returnsOnlyAllowedPermissions
}
assert WithoutAllowedPermissionCheckAndIssuableSubsetInvariantKeepsAllowedPermissions {
  ModelConfiguration.policy = WithoutAllowedPermissionCheckAndIssuableSubsetInvariant implies
    returnsOnlyAllowedPermissions
}
assert WithoutIssuablePermissionCheckPreservesIssuablePermissions {
  ModelConfiguration.policy = WithoutIssuablePermissionCheck implies
    (all t: ReturnableIssuanceResult |
      t.result.effectivePermissions in (levelInclusion[Config.issuablePermissions] + impliedPermissions))
}
assert WithoutOwnerCheckPreservesOwnerBoundary {
  ModelConfiguration.policy = WithoutOwnerCheck implies
    (all t: ReturnableIssuanceResult |
      all r: t.result.effectiveRepositories.repository | r.owner = Config.allowedOwner)
}
assert WithoutNameCheckPreservesExactNames {
  ModelConfiguration.policy = WithoutNameCheck implies
    (all t: ReturnableIssuanceResult |
      t.result.effectiveRepositories.repository.fullName = t.result.call.grant.request.repositories and
      #(t.result.effectiveRepositories) = #(t.result.call.grant.request.repositories))
}
assert WithoutCountCheckPreservesUniqueEffectiveRepositories {
  ModelConfiguration.policy = WithoutCountCheck implies
    (all t: ReturnableIssuanceResult | effectiveRepositoriesAreUnique[t.result])
}
assert WithoutExcessPermissionCheckPreservesPermissionBoundary {
  ModelConfiguration.policy = WithoutExcessPermissionCheck implies
    (all t: ReturnableIssuanceResult |
      t.result.effectivePermissions in (t.result.call.grant.request.permissions + impliedPermissions))
}
assert WithoutMissingPermissionCheckPreservesRequestedPermissions {
  ModelConfiguration.policy = WithoutMissingPermissionCheck implies
    (all t: ReturnableIssuanceResult |
      t.result.call.grant.request.permissions in t.result.effectivePermissions)
}
// Without the name check of Allowed Callers, the repository that a mistyped
// ID points to receives a protected token under another name.
assert WithoutAllowedCallerNameCheckPreservesAllowedCallers {
  ModelConfiguration.policy = WithoutAllowedCallerNameCheck implies protectedTokensOnlyToAllowedCallers
}
// Without the ID check of Allowed Callers, a Caller whose repository only has
// the name of an Allowed Caller receives a protected token.
assert WithoutAllowedCallerIdCheckPreservesAllowedCallers {
  ModelConfiguration.policy = WithoutAllowedCallerIdCheck implies protectedTokensOnlyToAllowedCallers
}
// Without the name check before issuance, a Caller that is no Allowed Caller
// receives a token for a protected repository whose ID and name match the list.
assert WithoutProtectedNameCheckPreservesAllowedCallers {
  ModelConfiguration.policy = WithoutProtectedNameCheck implies protectedTokensOnlyToAllowedCallers
}
// Without the ID check after issuance, a Caller that is no Allowed Caller
// receives a token for a protected repository requested by its new name.
assert WithoutProtectedIdCheckPreservesAllowedCallers {
  ModelConfiguration.policy = WithoutProtectedIdCheck implies protectedTokensOnlyToAllowedCallers
}
// Without the Permission Limit check, an Allowed Caller receives a protected
// token with a permission beyond its Permission Limit.
assert WithoutPermissionLimitCheckPreservesPermissionLimit {
  ModelConfiguration.policy = WithoutPermissionLimitCheck implies protectedTokensWithinPermissionLimit
}
// Without RULE-015 under enforcement, a Repository Policy alone grants a
// protected repository: the policy would override the list. RULE-015 is
// judged both for the Policy Read Grant and for the Token Grant, and the
// Token Grant needs a token of a Policy Read Grant of the same request, so
// the variant drops it from both; dropping it from one leaves the other.
assert WithoutProtectedCheckWhenEnforcedOnlyAddsConditions {
  ModelConfiguration.policy = WithoutProtectedCheckWhenEnforced implies grantsMeetConditionsWithoutPolicies
}
assert WithoutProtectedCheckWhenEnforcedNeedsListAndPolicy {
  ModelConfiguration.policy = WithoutProtectedCheckWhenEnforced implies protectedGrantsNeedListAndPolicy
}
// Applying RULE-016 regardless of the enforcement makes a deployment without
// enforcement read Repository Policies and rely on them for its grants.
assert WithoutEnforcementSwitchIgnoresRepositoryPolicies {
  ModelConfiguration.policy = WithoutEnforcementSwitch implies decisionIgnoresPoliciesWhenNotEnforced
}
// The decision then depends on Repository Policies: with no name race during
// the read, a deployment without enforcement returns no token for a
// repository without one, unlike `NotEnforcedTokenWithoutPolicy`.
assert WithoutEnforcementSwitchDeniesWithoutPolicy {
  (ModelConfiguration.policy = WithoutEnforcementSwitch and not enforced) implies
    (no t: ReturnableIssuanceResult |
      (all f: t.result.call.grant.policyToken.~token | fetchReadsTokenRepository[f]) and
      some x: t.result.effectiveRepositories.repository | x.policy = NoPolicy)
}
// Without RULE-016, a name without a valid Repository Policy is granted, and
// the list alone decides a protected repository.
assert WithoutRepositoryPolicyCheckDeniesByDefault {
  ModelConfiguration.policy = WithoutRepositoryPolicyCheck implies grantsNeedValidPolicies
}
assert WithoutRepositoryPolicyCheckNeedsListAndPolicy {
  ModelConfiguration.policy = WithoutRepositoryPolicyCheck implies protectedGrantsNeedListAndPolicy
}
// Checking one name instead of every name grants a request with a name
// whose Repository Policy does not allow it.
assert WithoutPolicyAllTargetsNeedsEveryTarget {
  ModelConfiguration.policy = WithoutPolicyAllTargets implies grantsNeedEveryTargetAllowed
}
// Without the Permission Limit of the Repository Policy, a grant exceeds it.
assert WithoutPolicyPermissionLimitCheckKeepsLimit {
  ModelConfiguration.policy = WithoutPolicyPermissionLimitCheck implies grantsWithinEveryPolicyLimit
}
// The same variant returns a token beyond that Permission Limit.
assert WithoutPolicyPermissionLimitCheckKeepsReturnedLimit {
  ModelConfiguration.policy = WithoutPolicyPermissionLimitCheck implies policyTokensWithinPermissionLimit
}
// Dropping either half of the shared Allowed Caller condition lets a Caller
// matching only the other half be granted under a Repository Policy.
assert WithoutAllowedCallerNameCheckNeedsIdAndName {
  ModelConfiguration.policy = WithoutAllowedCallerNameCheck implies policyGrantsNeedIdAndName
}
assert WithoutAllowedCallerIdCheckNeedsIdAndName {
  ModelConfiguration.policy = WithoutAllowedCallerIdCheck implies policyGrantsNeedIdAndName
}
// Without the Repository ID check against the Policy Read Token, a token for
// a repository re-created after the read is returned under the Repository
// Policy of the repository it replaced.
assert WithoutPolicyRepositoryIdCheckReturnsPolicyRepositories {
  ModelConfiguration.policy = WithoutPolicyRepositoryIdCheck implies returnedRepositoriesArePolicyRepositories
}
assert WithoutPolicyRepositoryIdCheckAllowedByOwnPolicy {
  ModelConfiguration.policy = WithoutPolicyRepositoryIdCheck implies returnedRepositoriesAllowedByOwnPolicy
}
// Without the permission check on the Policy Read Token, a token with more
// than `single_file:read` is used.
assert WithoutPolicyReadPermissionCheckOnlyReadsThePolicy {
  ModelConfiguration.policy = WithoutPolicyReadPermissionCheck implies policyReadTokensOnlyReadThePolicy
}
// Without the path check on the Policy Read Token, a token that reads files
// other than the Repository Policy is used.
assert WithoutPolicyReadPathCheckOnlyReadsThePolicy {
  ModelConfiguration.policy = WithoutPolicyReadPathCheck implies policyReadTokensOnlyReadThePolicy
}
// Without the other conditions on the Policy Read Grant, a Policy Read Token
// is issued for a request the pre-authorization rejects.
assert WithoutPolicyReadAuthorizationMeetsOtherConditions {
  ModelConfiguration.policy = WithoutPolicyReadAuthorization implies policyReadGrantsMeetOtherConditions
}

// Positive witness: successful issuance must be satisfiable, or assertions
// could pass vacuously in an inconsistent model.
pred SuccessfulReturn {
  baseline
  some t: ReturnableIssuanceResult |
    githubResultRespectsInstallation[t.result]
}

// Positive witness for the widened permissions: a token with several
// permissions, one of them at `write`, can be returned.
pred WriteAndMultiplePermissionsReturned {
  baseline
  some t: ReturnableIssuanceResult |
    githubResultRespectsInstallation[t.result] and
    #(t.result.call.grant.permissions) > 1 and
    Write in t.result.call.grant.permissions.level
}

// The issuable permissions can be narrower than the allowed permissions while
// the Installation holds an allowed permission outside them, and a token is
// still returned for a request within the issuable permissions.
pred IssuableNarrowerThanInstallation {
  baseline
  some t: ReturnableIssuanceResult |
    githubResultRespectsInstallation[t.result] and
    some (allowedPermissions - levelInclusion[Config.issuablePermissions] - impliedPermissions)
      & t.result.call.grant.installation.installedPermissions
}

// Issuable permissions holding only `write` of a name return a `read` token
// of that name at `read`.
pred IssuableWriteAdmitsRead {
  baseline
  some t: ReturnableIssuanceResult, e: t.result.call.grant.permissions |
    githubResultRespectsInstallation[t.result] and
    e.level = Read and
    (no x: Config.issuablePermissions | x.name = e.name and x.level = Read) and
    (some x: Config.issuablePermissions | x.name = e.name and x.level = Write)
}

// The implied `metadata:read` added by GitHub is returned even when the
// issuable permissions do not include it.
pred ImpliedPermissionOutsideIssuableReturned {
  baseline
  no impliedPermissions & levelInclusion[Config.issuablePermissions]
  some t: ReturnableIssuanceResult |
    githubResultRespectsInstallation[t.result] and
    impliedPermissions in t.result.effectivePermissions
}

// Witnesses for the checks that remove guards: tokens are still returned
// under each policy, so those checks do not pass vacuously. Without the
// invariant, the issuable permissions can reach outside the allowed ones.
pred ReturnWithoutAllowedPermissionCheck {
  ModelConfiguration.policy = WithoutAllowedPermissionCheck
  some ReturnableIssuanceResult
}
pred ReturnWithoutInclusionAllowedCondition {
  ModelConfiguration.policy = WithoutInclusionAllowedCondition
  some ReturnableIssuanceResult
}
pred ReturnWithoutIssuableSubsetInvariant {
  ModelConfiguration.policy = WithoutIssuableSubsetInvariant
  some Config.issuablePermissions - allowedPermissions
  some ReturnableIssuanceResult
}
pred ReturnWithoutAllowedPermissionCheckAndIssuableSubsetInvariant {
  ModelConfiguration.policy = WithoutAllowedPermissionCheckAndIssuableSubsetInvariant
  some Config.issuablePermissions - allowedPermissions
  some ReturnableIssuanceResult
}
// The variant applying RULE-016 regardless of the enforcement still returns a
// token without enforcement and without a name race, so
// `WithoutEnforcementSwitchDeniesWithoutPolicy` does not pass vacuously.
pred ReturnWithoutEnforcementSwitch {
  ModelConfiguration.policy = WithoutEnforcementSwitch
  not enforced
  some t: ReturnableIssuanceResult |
    all f: t.result.call.grant.policyToken.~token | fetchReadsTokenRepository[f]
}

// Intended residual risk without Repository Policy Enforcement: a caller's
// repository is DIFFERENT from the requested target, but both are within the
// same allowed owner. Without enforcement, no per-caller access control
// applies outside the protected repositories.
pred CrossRepositoryWithinOwner {
  baseline
  not enforced
  some t: ReturnableIssuanceResult, target: Repository |
    githubResultRespectsInstallation[t.result] and
    target in t.result.effectiveRepositories.repository and
    target != t.result.call.grant.caller.repository and
    target.fullName != t.result.call.grant.caller.repository.fullName
}

// Exploration of the trust boundary: without assuming GitHub validates the
// installation's coverage, result fields could appear self-consistent to
// GATES even when coverage is not proved by GATES. Such a witness does not
// establish exploitable GitHub API behavior.
pred IssueCallMayContainUncoveredNames {
  baseline
  some c: IssueCall |
    some n: c.repositories |
      no r: c.grant.installation.coveredRepositories | r.fullName = n
}

// GATES sends the requested permissions without checking that the
// Installation holds them (security.md 3.3).
pred IssueCallMayContainUnheldPermissions {
  baseline
  some c: IssueCall |
    c.permissions not in (coveredBy[c.grant.installation.installedPermissions] + impliedPermissions)
}

// GitHub may answer a request with a permission the Installation does not hold
// by issuing without it. Such a result stays within the Installation, and the
// permission checks keep it from being returned.
pred ResultWithoutUnheldPermissionNotReturned {
  baseline
  some r: IssuanceResult |
    githubResultRespectsInstallation[r] and
    (some e: r.call.grant.permissions |
      e not in (coveredBy[r.call.grant.installation.installedPermissions] + impliedPermissions) and
      no x: r.effectivePermissions | x.name = e.name) and
    no t: ReturnableIssuanceResult | t.result = r
}

pred CoverageNotIndependentlyProved {
  baseline
  some t: ReturnableIssuanceResult |
    t.result.effectiveRepositories.repository not in t.result.call.grant.installation.coveredRepositories
}

// Witness that an invalid result can exist independently of return.
// Non-return of overbroad results is checked by NoOverbroadTokenReturned.
pred InvalidResultCanExist {
  baseline
  some r: IssuanceResult |
    r.effectivePermissions not in (r.call.grant.permissions + impliedPermissions) and
    no t: ReturnableIssuanceResult | t.result = r
}

// An Allowed Caller receives a token for its protected repository.
pred AllowedCallerReceivesProtectedToken {
  baseline
  some t: ReturnableIssuanceResult, p: Config.protectedRepositories |
    githubResultRespectsInstallation[t.result] and
    p.repository in t.result.effectiveRepositories.repository
}

// An Allowed Caller whose Permission Limit holds only `write` of a name
// receives a token with `read` of that name.
pred WriteLimitAdmitsReadToken {
  baseline
  some t: ReturnableIssuanceResult, p: Config.protectedRepositories, a: p.allowedCallers,
       e: t.result.call.grant.permissions |
    githubResultRespectsInstallation[t.result] and
    p.repository in t.result.effectiveRepositories.repository and
    isAllowedCaller[t.result.call.grant.caller, a] and
    e.level = Read and
    (no x: a.permissionLimit | x.name = e.name and x.level = Read) and
    (some x: a.permissionLimit | x.name = e.name and x.level = Write)
}

// A Permission Limit may hold entries outside the issuable permissions, and
// the Allowed Caller still receives a token within them.
pred PermissionLimitBeyondIssuableReturnsWithinIssuable {
  baseline
  some t: ReturnableIssuanceResult, p: Config.protectedRepositories, a: p.allowedCallers |
    githubResultRespectsInstallation[t.result] and
    p.repository in t.result.effectiveRepositories.repository and
    isAllowedCaller[t.result.call.grant.caller, a] and
    a.permissionLimit not in levelInclusion[Config.issuablePermissions]
}

// Intended behavior of Current Scope without Repository Policies: with
// protected repositories in place, a Caller that is no Allowed Caller of any
// of them receives a token for a repository not in the list.
pred UnlistedRepositoryTokenToAnyCaller {
  baseline
  not enforced
  some Config.protectedRepositories
  some t: ReturnableIssuanceResult |
    githubResultRespectsInstallation[t.result] and
    (no t.result.effectiveRepositories.repository & Config.protectedRepositories.repository) and
    no a: Config.protectedRepositories.allowedCallers |
      a.repository = t.result.call.grant.caller.repository
}

// A renamed protected repository requested by its new name passes the checks
// before issuance and reaches the issuance API, so GitHub may issue a token.
// The result meets every check against the grant, and the ID check alone
// keeps it from being returned.
pred RenamedProtectedRepositoryIssuedButNotReturned {
  baseline
  some r: IssuanceResult, p: Config.protectedRepositories |
    githubResultRespectsInstallation[r] and
    p.repository in r.effectiveRepositories.repository and
    p.repository.fullName.repositoryName != p.repositoryName and
    p.repository.fullName in r.call.grant.repositories and
    resultMatchesGrant[r] and
    not verifiedResult[r]
}

// A request mixing a protected repository and an unprotected one returns one
// token for both to an Allowed Caller.
pred MixedTargetsReturnedToAllowedCaller {
  baseline
  some t: ReturnableIssuanceResult, p: Config.protectedRepositories, x: Repository |
    githubResultRespectsInstallation[t.result] and
    p.repository in t.result.effectiveRepositories.repository and
    x in t.result.effectiveRepositories.repository and
    x not in Config.protectedRepositories.repository
}

// INV-016 allows Allowed Callers of different protected repositories to share
// an ID with different names. A token is still returned for one of them.
pred SharedAllowedCallerIdWithDifferentNames {
  baseline
  some disj p, q: Config.protectedRepositories, a: p.allowedCallers, b: q.allowedCallers,
       t: ReturnableIssuanceResult |
    a.repository = b.repository and a.repositoryName != b.repositoryName and
    githubResultRespectsInstallation[t.result] and
    p.repository in t.result.effectiveRepositories.repository
}

// With enforcement, a token is returned for a repository whose Repository
// Policy allows the Caller.
pred PolicyAllowedTokenReturned {
  baseline
  enforced
  some t: ReturnableIssuanceResult | githubResultRespectsInstallation[t.result]
}

// With enforcement, an Allowed Caller of both the list and the Repository
// Policy receives a token for a protected repository.
pred PolicyAllowedProtectedTokenReturned {
  baseline
  enforced
  some t: ReturnableIssuanceResult, p: Config.protectedRepositories |
    githubResultRespectsInstallation[t.result] and
    p.repository in t.result.effectiveRepositories.repository
}

// A Caller requesting its own repository goes through the Repository Policy
// too, and receives a token when the policy allows it.
pred OwnRepositoryNeedsPolicy {
  baseline
  enforced
  some t: ReturnableIssuanceResult |
    githubResultRespectsInstallation[t.result] and
    t.result.call.grant.caller.repository in t.result.effectiveRepositories.repository
}

// A repository re-created under the same name after its Repository Policy
// was read: the issuance reaches GitHub and the result meets every other
// check, and the Repository ID check alone keeps it from being returned.
pred RecreatedTargetIssuedButNotReturned {
  baseline
  enforced
  some r: IssuanceResult |
    githubResultRespectsInstallation[r] and
    resultMatchesGrant[r] and protectedRepositoriesMatchResult[r] and
    not verifiedResult[r]
}

// Accepted risk (security.md 3.7): if the name points to another repository
// during the read, GATES can read the Repository Policy of that repository
// and return a token for a repository whose own Repository Policy does not
// allow the Caller. Making it happen takes renaming or deleting the target,
// which also lets one rewrite its Repository Policy.
pred NameRaceDuringReadMayUseOtherPolicy {
  baseline
  enforced
  some t: ReturnableIssuanceResult, f: t.result.call.grant.policyToken.~token,
       x: t.result.effectiveRepositories.repository |
    not fetchReadsTokenRepository[f] and
    no a: x.policy.allowedCallers | isAllowedCaller[t.result.call.grant.caller, a]
}

// Without enforcement, a token is returned for a repository without a
// Repository Policy.
pred NotEnforcedTokenWithoutPolicy {
  baseline
  not enforced
  some t: ReturnableIssuanceResult |
    githubResultRespectsInstallation[t.result] and
    some x: t.result.effectiveRepositories.repository | x.policy = NoPolicy
}

// All commands use a finite scope; absence of counterexamples is NOT
// an unbounded proof.
// PermissionName is 5 and PermissionEntry is 10, so that every pair of the five
// names and two levels exists. Besides the fixed Metadata and SingleFile, three
// names can mix listed and unlisted ones.
check NoForeignOwnerGrant for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
check NoOtherInstallationGrant for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
check OnlyAllowedPermissionsGranted for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
// trace:checks FR-AUTHZ-008 RULE-010
check OnlyIssuablePermissionsGranted for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
check AllOrNothingGrant for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
check AllIssueCallsExplicit for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
check NoOverbroadTokenReturned for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
check NoNarrowedTokenReturned for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
check UnrequestedAllowedPermissionsNotReturned for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
// trace:checks FR-AUTHZ-008 RULE-010
check NoTokenBeyondIssuablePermissions for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
// trace:checks FR-AUTHZ-008 INV-011
check ReturnedLevelsEqualRequestedLevels for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
// trace:checks FR-AUTHZ-006 FR-AUTHZ-008 RULE-010
check IssuableWriteAdmitsNoReadOutsideAllowedPermissions for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
check OnlyRepositoryPermissionsReturned for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
check ExactNamesReturned for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
check NoDuplicateEffectiveRepositoriesReturned for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
check ReturnedReposOwnedByAllowedOwner for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
check CoverageHoldsWhenGitHubEnforces for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
check RequestedPermissionsHeldWhenGitHubEnforces for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
check ReturnedResultsHaveValidTokenAndExpiry for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
// trace:checks FR-AUTHZ-009 FR-AUTHZ-010 RULE-015 INV-012
check ProtectedTokenNeedsIdAndNameOfAllowedCaller for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
// trace:checks FR-AUTHZ-009 FR-AUTHZ-010 RULE-015 INV-012
check ProtectedTokenWithinPermissionLimit for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
// trace:checks FR-AUTHZ-010 INV-012
check NoProtectedNameWithOtherIdReturned for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
// trace:checks FR-AUTHZ-009 RULE-015 INV-012
check ProtectedRepositoriesAddNothingWhenNotTargeted for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
// trace:checks FR-AUTHZ-009 INV-016
check ProtectedRepositoriesKeepPermissionBoundary for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
// trace:checks FR-AUTHZ-011 RULE-016 INV-011
check RepositoryPoliciesOnlyAddConditions for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
// trace:checks FR-AUTHZ-011 FR-OPS-001 INV-011
check NotEnforcedIgnoresRepositoryPolicies for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
// trace:checks FR-AUTHZ-011 RULE-016 INV-017
check DefaultDenyWithoutValidPolicy for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
// trace:checks FR-AUTHZ-009 FR-AUTHZ-011 RULE-015 RULE-016
check ProtectedRepositoriesNeedListAndPolicy for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
// trace:checks FR-AUTHZ-011 FR-AUTHZ-004 RULE-016
check EveryTargetAllowedByPolicy for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
// trace:checks FR-AUTHZ-008 FR-AUTHZ-011 RULE-016 INV-011
check GrantWithinEveryPermissionLimit for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
// trace:checks FR-AUTHZ-011 RULE-016
check PolicyAllowedCallerNeedsIdAndName for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
// trace:checks FR-AUTHZ-012 INV-012
check ReturnedRepositoriesArePolicyRepositories for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
// trace:checks FR-AUTHZ-011 FR-AUTHZ-012 RULE-016 INV-012
check ReturnedRepositoriesAllowedByOwnPolicy for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
// trace:checks FR-AUTHZ-011 FR-ISSUANCE-004 RULE-016 INV-012
check PolicyTokenWithinPermissionLimit for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
// trace:checks FR-ISSUANCE-004 FR-ISSUANCE-007 INV-019
check PolicyReadTokenOnlyReadsThePolicy for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
// trace:checks FR-ISSUANCE-002 FR-ISSUANCE-007 INV-020
check PolicyReadGrantMeetsOtherConditions for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0

// These checks remove one of the two guards that keep returned tokens within
// the allowed permissions, or INV-015 that is not one of them. No
// counterexample is expected: a guard still holds.
check WithoutAllowedPermissionCheckKeepsAllowedPermissions for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
check WithoutInclusionAllowedConditionKeepsAllowedPermissions for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
check WithoutIssuableSubsetInvariantKeepsAllowedPermissions for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
check WithoutAllowedPermissionCheckAndIssuableSubsetInvariantKeepsAllowedPermissions for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0
// trace:checks FR-AUTHZ-011
check WithoutEnforcementSwitchDeniesWithoutPolicy for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 0

// These checks use a different verification policy from the baseline.
// A counterexample is expected: it shows that the removed check matters.
check WithoutBothAllowedPermissionGuardsPreservesAllowedPermissions for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
check WithoutIssuablePermissionCheckPreservesIssuablePermissions for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
check WithoutOwnerCheckPreservesOwnerBoundary for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
check WithoutNameCheckPreservesExactNames for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
check WithoutExcessPermissionCheckPreservesPermissionBoundary for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
check WithoutMissingPermissionCheckPreservesRequestedPermissions for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
check WithoutCountCheckPreservesUniqueEffectiveRepositories for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-009 RULE-015
check WithoutAllowedCallerNameCheckPreservesAllowedCallers for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-009 RULE-015
check WithoutAllowedCallerIdCheckPreservesAllowedCallers for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-009 FR-AUTHZ-010 RULE-015
check WithoutProtectedNameCheckPreservesAllowedCallers for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-010 INV-012
check WithoutProtectedIdCheckPreservesAllowedCallers for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-009 RULE-015
check WithoutPermissionLimitCheckPreservesPermissionLimit for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-011 RULE-016
check WithoutProtectedCheckWhenEnforcedOnlyAddsConditions for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-009 FR-AUTHZ-011 RULE-015 RULE-016
check WithoutProtectedCheckWhenEnforcedNeedsListAndPolicy for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-011 FR-OPS-001
check WithoutEnforcementSwitchIgnoresRepositoryPolicies for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-011 RULE-016
check WithoutRepositoryPolicyCheckDeniesByDefault for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-009 FR-AUTHZ-011 RULE-015 RULE-016
check WithoutRepositoryPolicyCheckNeedsListAndPolicy for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-011 FR-AUTHZ-004 RULE-016
check WithoutPolicyAllTargetsNeedsEveryTarget for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-011 RULE-016
check WithoutPolicyPermissionLimitCheckKeepsLimit for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-011 RULE-016
check WithoutPolicyPermissionLimitCheckKeepsReturnedLimit for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-011 RULE-016
check WithoutAllowedCallerNameCheckNeedsIdAndName for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-011 RULE-016
check WithoutAllowedCallerIdCheckNeedsIdAndName for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-012 INV-012
check WithoutPolicyRepositoryIdCheckReturnsPolicyRepositories for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-012 INV-012
check WithoutPolicyRepositoryIdCheckAllowedByOwnPolicy for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-ISSUANCE-007 INV-019
check WithoutPolicyReadPermissionCheckOnlyReadsThePolicy for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-ISSUANCE-004 INV-019
check WithoutPolicyReadPathCheckOnlyReadsThePolicy for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-ISSUANCE-002 INV-020
check WithoutPolicyReadAuthorizationMeetsOtherConditions for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1

run SuccessfulReturn for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
run WriteAndMultiplePermissionsReturned for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
run IssuableNarrowerThanInstallation for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-008 RULE-010
run IssuableWriteAdmitsRead for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
run ImpliedPermissionOutsideIssuableReturned for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
run ReturnWithoutAllowedPermissionCheck for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
run ReturnWithoutInclusionAllowedCondition for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
run ReturnWithoutIssuableSubsetInvariant for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
run ReturnWithoutAllowedPermissionCheckAndIssuableSubsetInvariant for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
run ReturnWithoutEnforcementSwitch for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
run CrossRepositoryWithinOwner for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
run IssueCallMayContainUncoveredNames for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
run IssueCallMayContainUnheldPermissions for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
run ResultWithoutUnheldPermissionNotReturned for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
run CoverageNotIndependentlyProved for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
run InvalidResultCanExist for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-009 RULE-015
run AllowedCallerReceivesProtectedToken for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-009 RULE-015
run WriteLimitAdmitsReadToken for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-009 INV-016
run PermissionLimitBeyondIssuableReturnsWithinIssuable for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-009 RULE-015
run UnlistedRepositoryTokenToAnyCaller for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-010 INV-012
run RenamedProtectedRepositoryIssuedButNotReturned for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-009 RULE-015
run MixedTargetsReturnedToAllowedCaller for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-009 INV-016
run SharedAllowedCallerIdWithDifferentNames for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-011 RULE-016
run PolicyAllowedTokenReturned for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-009 FR-AUTHZ-011 RULE-015 RULE-016
run PolicyAllowedProtectedTokenReturned for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-011 RULE-016
run OwnRepositoryNeedsPolicy for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-012 INV-012
run RecreatedTargetIssuedButNotReturned for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-012 RULE-016
run NameRaceDuringReadMayUseOtherPolicy for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
// trace:checks FR-AUTHZ-011
run NotEnforcedTokenWithoutPolicy for 3 but 26 VerificationPolicy, 4 PolicyState, 2 Owner, 2 OwnerName, 4 PermissionCategory, 5 PermissionName, 10 PermissionEntry expect 1
