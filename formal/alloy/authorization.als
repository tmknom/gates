module gates/authorization

// GATES authorization model, Current Scope.
// Source of truth: FR-AUTHZ-001..003, FR-AUTHZ-006..009, FR-ISSUANCE-003, FR-ISSUANCE-004, FR-ISSUANCE-006, FR-OPS-006,
// RULE-010, RULE-011, INV-002, INV-011, INV-012, INV-015;
// docs/design/security.md 3.1..3.5; docs/design/domain.md 3.3.
// This is a finite structural abstraction, NOT the complete GATES implementation.
// Cryptographic verification, HTTP outcomes, time, revocation, retry and
// configuration changes are outside this model.
// All Caller atoms represent already-verified GitHub OIDC identities.
// Names follow docs/design/domain.md. An Owner atom stands for one Owner ID and
// a Repository atom for one Repository ID, so field names omit the "ID" suffix.

sig Owner {}
sig OwnerName {}
sig FullName { ownerName: one OwnerName }
sig Repository { owner: one Owner, fullName: one FullName }

// The categories GitHub uses for GitHub App permissions. Only Repository
// permissions are limited by the repository-scoped authorization and
// verification; the others reach beyond the requested repositories.
abstract sig PermissionCategory {}
one sig RepositoryPermissions, OrganizationPermissions,
        AccountPermissions, EnterprisePermissions extends PermissionCategory {}

// Two levels stand for the levels a permission name can have. `admin` of
// `repository_projects` is not modeled.
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

// A held level also covers the lower level: an Installation granted `write`
// can issue `read` of the same name.
fun levelsCoveredBy[l: Level]: set Level {
  (l = Write) => (Read + Write) else Read
}
fun coveredBy[held: set PermissionEntry]: set PermissionEntry {
  { e: PermissionEntry | some h: held | h.name = e.name and e.level in levelsCoveredBy[h.level] }
}

abstract sig RepositorySelection {}
one sig Selected, AllRepositories extends RepositorySelection {}

abstract sig Answer {}
one sig Yes, No extends Answer {}

sig Caller { owner: one Owner, repository: one Repository }

sig Installation {
  installationOwner: one Owner,
  suspended: one Answer,
  coveredRepositories: set Repository,
  installedPermissions: set PermissionEntry
}

// The issuable permissions are fixed configuration chosen by the GATES
// operator for one deployment. Like the allowed permissions they are a set of
// Permission Entries, may hold several levels of one name, and do not depend
// on the Caller or the target repositories.
one sig Config {
  allowedOwner: one Owner,
  allowedOwnerName: one OwnerName,
  issuablePermissions: set PermissionEntry
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
// the same repository, so duplicates are representable.
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
// check of authorization, the invariant that the issuable permissions are
// part of the allowed permissions (INV-015, checked when the configuration is
// read), both of them, the issuable permissions check of authorization, or one
// return-time check.
// They are counterfactual verification models, not GATES configurations.
abstract sig VerificationPolicy {}
one sig Baseline, WithoutAllowedPermissionCheck, WithoutIssuableSubsetInvariant,
        WithoutBothAllowedPermissionGuards, WithoutIssuablePermissionCheck,
        WithoutOwnerCheck, WithoutNameCheck,
        WithoutExcessPermissionCheck, WithoutMissingPermissionCheck,
        WithoutCountCheck extends VerificationPolicy {}
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
  (ModelConfiguration.policy not in WithoutIssuableSubsetInvariant + WithoutBothAllowedPermissionGuards) implies
    Config.issuablePermissions in allowedPermissions
}

// Identity consistency comes from successful OIDC verification; it is not
// checked again by authorization. The caller's repository is NOT an ACL key.
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

// The conditions of the Grant operation: the authorization rule and the
// validity of the installation. The owner ID and owner name are fixed
// configuration; the owner name is modeled separately from the immutable
// owner ID. A request with any entry outside the allowed permissions or the
// issuable permissions is rejected as a whole, never narrowed. Entries are
// compared as pairs of name and level, so a higher issuable level does not
// admit a lower requested level.
pred grantable[c: Caller, r: TokenRequest, i: Installation] {
  c.owner = Config.allowedOwner
  all n: r.repositories | n.ownerName = Config.allowedOwnerName
  (ModelConfiguration.policy not in WithoutAllowedPermissionCheck + WithoutBothAllowedPermissionGuards) implies
    r.permissions in allowedPermissions
  (ModelConfiguration.policy != WithoutIssuablePermissionCheck) implies
    r.permissions in Config.issuablePermissions
  i.installationOwner = Config.allowedOwner
  i.suspended = No
}

// Represents the checks before the GitHub issuance API call. In particular,
// actual repo ownership / installation coverage and the Installation's granted
// permissions are NOT preconditions here.
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

// Represents return-time checks, NOT a guarantee about the actual GitHub
// token's authorization outside a trustworthy GitHub response.
// Permissions are compared as pairs of name and level, so an entry at a lower
// or higher level than granted fails one of the two permission checks.
pred verifiedResult[r: IssuanceResult] {
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
  baseline implies (all g: TokenGrant | g.permissions in Config.issuablePermissions)
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
// permissions do not hold it; every other returned entry is issuable.
assert NoTokenBeyondIssuablePermissions {
  baseline implies (all t: ReturnableIssuanceResult |
    t.result.effectivePermissions in (Config.issuablePermissions + impliedPermissions))
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

// Mutation checks intentionally evaluate the same returned-token properties
// under weaker verification policies. Their results must be examined as
// counterexamples; they are not assertions about the baseline implementation.
pred returnsOnlyAllowedPermissions {
  all t: ReturnableIssuanceResult | t.result.effectivePermissions in allowedPermissions
}
// Either of the two guards alone keeps returned tokens within the allowed
// permissions; removing both breaks the property.
assert WithoutAllowedPermissionCheckKeepsAllowedPermissions {
  ModelConfiguration.policy = WithoutAllowedPermissionCheck implies returnsOnlyAllowedPermissions
}
assert WithoutIssuableSubsetInvariantKeepsAllowedPermissions {
  ModelConfiguration.policy = WithoutIssuableSubsetInvariant implies returnsOnlyAllowedPermissions
}
assert WithoutBothAllowedPermissionGuardsPreservesAllowedPermissions {
  ModelConfiguration.policy = WithoutBothAllowedPermissionGuards implies returnsOnlyAllowedPermissions
}
assert WithoutIssuablePermissionCheckPreservesIssuablePermissions {
  ModelConfiguration.policy = WithoutIssuablePermissionCheck implies
    (all t: ReturnableIssuanceResult |
      t.result.effectivePermissions in (Config.issuablePermissions + impliedPermissions))
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
assert ReturnedResultsHaveValidTokenAndExpiry {
  all t: ReturnableIssuanceResult |
    t.result.nonemptyToken = Yes and t.result.futureExpiry = Yes
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
    some (allowedPermissions - Config.issuablePermissions - impliedPermissions)
      & t.result.call.grant.installation.installedPermissions
}

// The implied `metadata:read` added by GitHub is returned even when the
// issuable permissions do not hold it.
pred ImpliedPermissionOutsideIssuableReturned {
  baseline
  no impliedPermissions & Config.issuablePermissions
  some t: ReturnableIssuanceResult |
    githubResultRespectsInstallation[t.result] and
    impliedPermissions in t.result.effectivePermissions
}

// Witnesses for the checks that remove one guard: tokens are still returned
// under each policy, so those checks do not pass vacuously. Without the
// invariant, the issuable permissions can reach outside the allowed ones.
pred ReturnWithoutAllowedPermissionCheck {
  ModelConfiguration.policy = WithoutAllowedPermissionCheck
  some ReturnableIssuanceResult
}
pred ReturnWithoutIssuableSubsetInvariant {
  ModelConfiguration.policy = WithoutIssuableSubsetInvariant
  some Config.issuablePermissions - allowedPermissions
  some ReturnableIssuanceResult
}

// Intended residual risk: a caller's repository is DIFFERENT from the
// requested target, but both are within the same allowed owner. No per-caller
// access control is modeled, by design for Current Scope.
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

// All commands use a finite scope; absence of counterexamples is NOT
// an unbounded proof.
// PermissionName is 4 and PermissionEntry is 8, so that every pair of the four
// names and two levels exists, and the names can mix listed and unlisted ones.
check NoForeignOwnerGrant for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check NoOtherInstallationGrant for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check OnlyAllowedPermissionsGranted for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check OnlyIssuablePermissionsGranted for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check AllOrNothingGrant for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check AllIssueCallsExplicit for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check NoOverbroadTokenReturned for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check NoNarrowedTokenReturned for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check UnrequestedAllowedPermissionsNotReturned for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check NoTokenBeyondIssuablePermissions for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check OnlyRepositoryPermissionsReturned for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check ExactNamesReturned for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check NoDuplicateEffectiveRepositoriesReturned for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check ReturnedReposOwnedByAllowedOwner for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check CoverageHoldsWhenGitHubEnforces for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check RequestedPermissionsHeldWhenGitHubEnforces for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check ReturnedResultsHaveValidTokenAndExpiry for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0

// These checks remove one of the two guards that keep returned tokens within
// the allowed permissions. No counterexample is expected: the other guard
// still holds.
check WithoutAllowedPermissionCheckKeepsAllowedPermissions for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0
check WithoutIssuableSubsetInvariantKeepsAllowedPermissions for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 0

// These checks use a different verification policy from the baseline.
// A counterexample is expected: it shows that the removed check matters.
check WithoutBothAllowedPermissionGuardsPreservesAllowedPermissions for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
check WithoutIssuablePermissionCheckPreservesIssuablePermissions for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
check WithoutOwnerCheckPreservesOwnerBoundary for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
check WithoutNameCheckPreservesExactNames for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
check WithoutExcessPermissionCheckPreservesPermissionBoundary for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
check WithoutMissingPermissionCheckPreservesRequestedPermissions for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
check WithoutCountCheckPreservesUniqueEffectiveRepositories for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1

run SuccessfulReturn for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
run WriteAndMultiplePermissionsReturned for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
run IssuableNarrowerThanInstallation for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
run ImpliedPermissionOutsideIssuableReturned for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
run ReturnWithoutAllowedPermissionCheck for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
run ReturnWithoutIssuableSubsetInvariant for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
run CrossRepositoryWithinOwner for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
run IssueCallMayContainUncoveredNames for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
run IssueCallMayContainUnheldPermissions for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
run ResultWithoutUnheldPermissionNotReturned for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
run CoverageNotIndependentlyProved for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
run InvalidResultCanExist for 3 but 10 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionCategory, 4 PermissionName, 8 PermissionEntry expect 1
