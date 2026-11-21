module gates/authorization

// GATES authorization model, Current Scope.
// Source of truth: FR-AUTHZ-001..010, FR-ISSUANCE-003, FR-ISSUANCE-004, FR-ISSUANCE-005, FR-OPS-001,
// RULE-010, RULE-011, RULE-015, INV-002, INV-011, INV-012, INV-015, INV-016;
// docs/design/security.md 3.1..3.6; docs/design/domain.md Permission Levels and
// Protected Repositories.
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
// repository has another ID (a re-creation or a mistyped ID).
sig Repository { owner: one Owner, fullName: one FullName }

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
// fixed configuration too, and can be empty.
one sig Config {
  allowedOwner: one Owner,
  allowedOwnerName: one OwnerName,
  issuablePermissions: set PermissionEntry,
  protectedRepositories: set ProtectedRepository
}

sig TokenRequest {
  repositories: some FullName,
  permissions: some PermissionEntry
}

// `caller` and `request` trace where the grant came from; they are not part of
// its content, which is the installation, repositories and permissions.
sig TokenGrant {
  caller: one Caller,
  request: one TokenRequest,
  installation: one Installation,
  repositories: some FullName,
  permissions: some PermissionEntry
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
// issuable permissions check of authorization, one return-time check, or one
// check on the protected repositories.
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
        WithoutPermissionLimitCheck, WithoutProtectedIdCheck extends VerificationPolicy {}
one sig ModelConfiguration { policy: one VerificationPolicy }

pred baseline { ModelConfiguration.policy = Baseline }

// A single GitHub issuance API invocation can yield at most one modeled
// result, and one result can become at most one returnable issuance result.
fact ResultCardinality {
  all c: IssueCall | lone c.~call
  all r: IssuanceResult | lone r.~result
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

// Identity consistency comes from successful OIDC verification; it is not
// checked again by authorization. The caller's repository is NOT an ACL key
// outside the protected repositories.
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
// Name equal those of the Allowed Caller.
pred isAllowedCaller[c: Caller, a: AllowedCaller] {
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

// The conditions of the Grant operation (INV-011).
pred grantable[c: Caller, r: TokenRequest, i: Installation] {
  ownerAndPermissionsGrantable[c, r, i]
  protectedRepositoriesAuthorized[c, r]
}

// Represents the checks before the GitHub issuance API call. In particular,
// actual repo ownership / installation coverage and the Installation's granted
// permissions are NOT preconditions here. The grant keeps the requested
// levels, never the higher levels of a limit.
// trace:formalizes INV-011
fact GrantsRespectAuthorization {
  all g: TokenGrant |
    grantable[g.caller, g.request, g.installation] and
    g.repositories = g.request.repositories and
    g.permissions = g.request.permissions
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
  all x: r.effectiveRepositories.repository, p: Config.protectedRepositories |
    x.fullName.repositoryName = p.repositoryName iff x = p.repository
}

// Represents return-time checks (INV-012), NOT a guarantee about the actual
// GitHub token's authorization outside a trustworthy GitHub response.
pred verifiedResult[r: IssuanceResult] {
  resultMatchesGrant[r]
  (ModelConfiguration.policy != WithoutProtectedIdCheck) implies
    protectedRepositoriesMatchResult[r]
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
      (all r: IssuanceResult | verifiedResult[r] iff resultMatchesGrant[r])
  }
}
// A Permission Limit outside the issuable or the allowed permissions does
// not widen what is returned.
assert ProtectedRepositoriesKeepPermissionBoundary {
  baseline implies (all t: ReturnableIssuanceResult |
    t.result.effectivePermissions in
      ((allowedPermissions & levelInclusion[Config.issuablePermissions]) + impliedPermissions))
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

// Intended residual risk: a caller's repository is DIFFERENT from the
// requested target, but both are within the same allowed owner. No per-caller
// access control is modeled outside the protected repositories, by design for
// Current Scope.
pred CrossRepositoryWithinOwner {
  baseline
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

// Intended behavior of Current Scope: with protected repositories in place, a
// Caller that is no Allowed Caller of any of them receives a token for a
// repository not in the list.
pred UnlistedRepositoryTokenToAnyCaller {
  baseline
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

// All commands use a finite scope; absence of counterexamples is NOT
// an unbounded proof.
// PermissionName is 4 and PermissionEntry is 8, so that every pair of the four
// names and two levels exists, and the names can mix listed and unlisted ones.
check NoForeignOwnerGrant for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check NoOtherInstallationGrant for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check OnlyAllowedPermissionsGranted for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
// trace:checks FR-AUTHZ-008 RULE-010
check OnlyIssuablePermissionsGranted for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check AllOrNothingGrant for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check AllIssueCallsExplicit for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check NoOverbroadTokenReturned for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check NoNarrowedTokenReturned for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check UnrequestedAllowedPermissionsNotReturned for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
// trace:checks FR-AUTHZ-008 RULE-010
check NoTokenBeyondIssuablePermissions for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
// trace:checks FR-AUTHZ-008 INV-011
check ReturnedLevelsEqualRequestedLevels for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
// trace:checks FR-AUTHZ-006 FR-AUTHZ-008 RULE-010
check IssuableWriteAdmitsNoReadOutsideAllowedPermissions for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check OnlyRepositoryPermissionsReturned for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check ExactNamesReturned for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check NoDuplicateEffectiveRepositoriesReturned for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check ReturnedReposOwnedByAllowedOwner for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check CoverageHoldsWhenGitHubEnforces for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check RequestedPermissionsHeldWhenGitHubEnforces for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check ReturnedResultsHaveValidTokenAndExpiry for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
// trace:checks FR-AUTHZ-009 FR-AUTHZ-010 RULE-015 INV-012
check ProtectedTokenNeedsIdAndNameOfAllowedCaller for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
// trace:checks FR-AUTHZ-009 FR-AUTHZ-010 RULE-015 INV-012
check ProtectedTokenWithinPermissionLimit for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
// trace:checks FR-AUTHZ-010 INV-012
check NoProtectedNameWithOtherIdReturned for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
// trace:checks FR-AUTHZ-009 RULE-015 INV-012
check ProtectedRepositoriesAddNothingWhenNotTargeted for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
// trace:checks FR-AUTHZ-009 INV-016
check ProtectedRepositoriesKeepPermissionBoundary for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0

// These checks remove one of the two guards that keep returned tokens within
// the allowed permissions, or INV-015 that is not one of them. No
// counterexample is expected: a guard still holds.
check WithoutAllowedPermissionCheckKeepsAllowedPermissions for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check WithoutInclusionAllowedConditionKeepsAllowedPermissions for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check WithoutIssuableSubsetInvariantKeepsAllowedPermissions for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check WithoutAllowedPermissionCheckAndIssuableSubsetInvariantKeepsAllowedPermissions for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0

// These checks use a different verification policy from the baseline.
// A counterexample is expected: it shows that the removed check matters.
check WithoutBothAllowedPermissionGuardsPreservesAllowedPermissions for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
check WithoutIssuablePermissionCheckPreservesIssuablePermissions for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
check WithoutOwnerCheckPreservesOwnerBoundary for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
check WithoutNameCheckPreservesExactNames for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
check WithoutExcessPermissionCheckPreservesPermissionBoundary for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
check WithoutMissingPermissionCheckPreservesRequestedPermissions for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
check WithoutCountCheckPreservesUniqueEffectiveRepositories for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
// trace:checks FR-AUTHZ-009 RULE-015
check WithoutAllowedCallerNameCheckPreservesAllowedCallers for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
// trace:checks FR-AUTHZ-009 FR-AUTHZ-010 RULE-015
check WithoutProtectedNameCheckPreservesAllowedCallers for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
// trace:checks FR-AUTHZ-010 INV-012
check WithoutProtectedIdCheckPreservesAllowedCallers for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
// trace:checks FR-AUTHZ-009 RULE-015
check WithoutPermissionLimitCheckPreservesPermissionLimit for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1

run SuccessfulReturn for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
run WriteAndMultiplePermissionsReturned for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
run IssuableNarrowerThanInstallation for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
// trace:checks FR-AUTHZ-008 RULE-010
run IssuableWriteAdmitsRead for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
run ImpliedPermissionOutsideIssuableReturned for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
run ReturnWithoutAllowedPermissionCheck for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
run ReturnWithoutInclusionAllowedCondition for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
run ReturnWithoutIssuableSubsetInvariant for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
run ReturnWithoutAllowedPermissionCheckAndIssuableSubsetInvariant for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
run CrossRepositoryWithinOwner for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
run IssueCallMayContainUncoveredNames for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
run IssueCallMayContainUnheldPermissions for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
run ResultWithoutUnheldPermissionNotReturned for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
run CoverageNotIndependentlyProved for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
run InvalidResultCanExist for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
// trace:checks FR-AUTHZ-009 RULE-015
run AllowedCallerReceivesProtectedToken for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
// trace:checks FR-AUTHZ-009 RULE-015
run WriteLimitAdmitsReadToken for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
// trace:checks FR-AUTHZ-009 INV-016
run PermissionLimitBeyondIssuableReturnsWithinIssuable for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
// trace:checks FR-AUTHZ-009 RULE-015
run UnlistedRepositoryTokenToAnyCaller for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
// trace:checks FR-AUTHZ-010 INV-012
run RenamedProtectedRepositoryIssuedButNotReturned for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
// trace:checks FR-AUTHZ-009 RULE-015
run MixedTargetsReturnedToAllowedCaller for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
// trace:checks FR-AUTHZ-009 INV-016
run SharedAllowedCallerIdWithDifferentNames for 3 but 16 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
