module gates/authorization

// GATES authorization model, Current Scope.
// Source of truth: docs/spec/requirements.md FR-006..008, FR-010..012, FR-015,
// FR-016, FR-018, FR-024;
// docs/design/security.md 3.1..3.4; docs/design/domain.md 3.3, 5.2..5.3.
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

abstract sig PermissionEntry {}
one sig ContentsRead, MetadataRead, ContentsWrite extends PermissionEntry {}
sig OtherPermissionEntry extends PermissionEntry {}

fun allowedPermissions: set PermissionEntry { ContentsRead }
fun impliedPermissions: set PermissionEntry { MetadataRead }

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

one sig Config {
  allowedOwner: one Owner,
  allowedOwnerName: one OwnerName
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

sig InstallationToken { result: one IssuanceResult }

// The baseline policy is used by the original checks and witnesses.
// The other policies deliberately disable exactly one return-time check.
// They are counterfactual verification models, not GATES configurations.
abstract sig VerificationPolicy {}
one sig Baseline, WithoutOwnerCheck, WithoutNameCheck,
        WithoutPermissionCheck, WithoutCountCheck extends VerificationPolicy {}
one sig ModelConfiguration { policy: one VerificationPolicy }

pred baseline { ModelConfiguration.policy = Baseline }

// A single GitHub issuance API invocation can yield at most one modeled
// result, and one result can become at most one installation token.
fact ResultCardinality {
  all c: IssueCall | lone c.~call
  all r: IssuanceResult | lone r.~result
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
// owner ID.
pred grantable[c: Caller, r: TokenRequest, i: Installation] {
  c.owner = Config.allowedOwner
  all n: r.repositories | n.ownerName = Config.allowedOwnerName
  r.permissions = allowedPermissions
  i.installationOwner = Config.allowedOwner
  i.suspended = No
}

// Represents the checks before the GitHub issuance API call. In particular,
// actual repo ownership / installation coverage is NOT a precondition here.
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
pred verifiedResult[r: IssuanceResult] {
  r.nonemptyToken = Yes
  r.futureExpiry = Yes
  r.repositorySelection = Selected
  ContentsRead in r.effectivePermissions

  // Equal count and equal name set leave no room for a repeated name or ID.
  (ModelConfiguration.policy != WithoutCountCheck) implies
    #(r.effectiveRepositories) = #(r.call.grant.repositories)
  (ModelConfiguration.policy != WithoutNameCheck) implies
    r.effectiveRepositories.repository.fullName = r.call.grant.repositories
  (ModelConfiguration.policy != WithoutOwnerCheck) implies
    (all x: r.effectiveRepositories.repository | x.owner = Config.allowedOwner)
  (ModelConfiguration.policy != WithoutPermissionCheck) implies
    r.effectivePermissions in (allowedPermissions + impliedPermissions)
}

fact OnlyVerifiedResultsBecomeTokens {
  all t: InstallationToken | verifiedResult[t.result]
}

// The following is an *external GitHub behavior assumption*, deliberately
// not a fact: GitHub must enforce installation coverage and app permissions.
// GATES does not prevalidate the requested names against coverage.
pred githubResultRespectsInstallation[r: IssuanceResult] {
  r.effectiveRepositories.repository in r.call.grant.installation.coveredRepositories
  r.effectivePermissions in (r.call.grant.installation.installedPermissions + impliedPermissions)
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
assert OnlyCurrentPermissionGranted {
  all g: TokenGrant | g.permissions = allowedPermissions
}
assert AllOrNothingGrant {
  all g: TokenGrant | g.repositories = g.request.repositories
}
assert AllIssueCallsExplicit {
  all c: IssueCall |
    c.repositories = c.grant.request.repositories and
    c.permissions = allowedPermissions
}
assert NoOverbroadTokenReturned {
  baseline implies (all t: InstallationToken |
    t.result.effectivePermissions in (allowedPermissions + impliedPermissions) and
    ContentsRead in t.result.effectivePermissions and
    t.result.repositorySelection = Selected)
}
assert ExactNamesReturned {
  baseline implies (all t: InstallationToken |
    t.result.effectiveRepositories.repository.fullName = t.result.call.grant.request.repositories and
    #(t.result.effectiveRepositories) = #(t.result.call.grant.request.repositories))
}
assert NoDuplicateEffectiveRepositoriesReturned {
  baseline implies (all t: InstallationToken | effectiveRepositoriesAreUnique[t.result])
}
assert ReturnedReposOwnedByAllowedOwner {
  baseline implies (all t: InstallationToken |
    all r: t.result.effectiveRepositories.repository | r.owner = Config.allowedOwner)
}
// Conditional on the external assumption: GATES does not prevalidate coverage,
// and its full-name check rejects a result that omits an uncovered name.
assert CoverageHoldsWhenGitHubEnforces {
  baseline implies (all t: InstallationToken |
    githubResultRespectsInstallation[t.result] implies
    (all n: t.result.call.grant.repositories |
      some r: t.result.call.grant.installation.coveredRepositories |
        r.fullName = n and r.owner = Config.allowedOwner))
}

// Mutation checks intentionally evaluate the same returned-token properties
// under weaker verification policies. Their results must be examined as
// counterexamples; they are not assertions about the baseline implementation.
assert WithoutOwnerCheckPreservesOwnerBoundary {
  ModelConfiguration.policy = WithoutOwnerCheck implies
    (all t: InstallationToken |
      all r: t.result.effectiveRepositories.repository | r.owner = Config.allowedOwner)
}
assert WithoutNameCheckPreservesExactNames {
  ModelConfiguration.policy = WithoutNameCheck implies
    (all t: InstallationToken |
      t.result.effectiveRepositories.repository.fullName = t.result.call.grant.request.repositories and
      #(t.result.effectiveRepositories) = #(t.result.call.grant.request.repositories))
}
assert WithoutCountCheckPreservesUniqueEffectiveRepositories {
  ModelConfiguration.policy = WithoutCountCheck implies
    (all t: InstallationToken | effectiveRepositoriesAreUnique[t.result])
}
assert WithoutPermissionCheckPreservesPermissionBoundary {
  ModelConfiguration.policy = WithoutPermissionCheck implies
    (all t: InstallationToken |
      t.result.effectivePermissions in (allowedPermissions + impliedPermissions) and
      ContentsRead in t.result.effectivePermissions and
      t.result.repositorySelection = Selected)
}
assert ReturnedResultsHaveValidTokenAndExpiry {
  all t: InstallationToken |
    t.result.nonemptyToken = Yes and t.result.futureExpiry = Yes
}

// Positive witness: successful issuance must be satisfiable, or assertions
// could pass vacuously in an inconsistent model.
pred SuccessfulReturn {
  baseline
  some t: InstallationToken |
    githubResultRespectsInstallation[t.result]
}

// Intended residual risk: a caller's repository is DIFFERENT from the
// requested target, but both are within the same allowed owner. No per-caller
// access control is modeled, by design for Current Scope.
pred CrossRepositoryWithinOwner {
  baseline
  some t: InstallationToken, target: Repository |
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

pred CoverageNotIndependentlyProved {
  baseline
  some t: InstallationToken |
    t.result.effectiveRepositories.repository not in t.result.call.grant.installation.coveredRepositories
}

// Witness that an invalid result can exist independently of return.
// Non-return of overbroad results is checked by NoOverbroadTokenReturned.
pred InvalidResultCanExist {
  baseline
  some r: IssuanceResult |
    ContentsWrite in r.effectivePermissions and
    no t: InstallationToken | t.result = r
}

// All commands use a finite scope; absence of counterexamples is NOT
// an unbounded proof.
// PermissionEntry is 4 so that one unknown entry exists besides the three named ones.
check NoForeignOwnerGrant for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionEntry expect 0
check NoOtherInstallationGrant for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionEntry expect 0
check OnlyCurrentPermissionGranted for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionEntry expect 0
check AllOrNothingGrant for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionEntry expect 0
check AllIssueCallsExplicit for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionEntry expect 0
check NoOverbroadTokenReturned for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionEntry expect 0
check ExactNamesReturned for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionEntry expect 0
check NoDuplicateEffectiveRepositoriesReturned for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionEntry expect 0
check ReturnedReposOwnedByAllowedOwner for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionEntry expect 0
check CoverageHoldsWhenGitHubEnforces for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionEntry expect 0
check ReturnedResultsHaveValidTokenAndExpiry for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionEntry expect 0

// These checks use a different verification policy from the baseline.
// A counterexample is expected: it shows that the removed check matters.
check WithoutOwnerCheckPreservesOwnerBoundary for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionEntry expect 1
check WithoutNameCheckPreservesExactNames for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionEntry expect 1
check WithoutPermissionCheckPreservesPermissionBoundary for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionEntry expect 1
check WithoutCountCheckPreservesUniqueEffectiveRepositories for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionEntry expect 1

run SuccessfulReturn for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionEntry expect 1
run CrossRepositoryWithinOwner for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionEntry expect 1
run IssueCallMayContainUncoveredNames for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionEntry expect 1
run CoverageNotIndependentlyProved for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionEntry expect 1
run InvalidResultCanExist for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 PermissionEntry expect 1
