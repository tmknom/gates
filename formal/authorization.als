module gates/authorization

// GATES authorization model, Current Scope.
// Source of truth: docs/spec/requirements.md FR-006..008, FR-010..012, FR-015,
// FR-016, FR-018, FR-024;
// docs/design/security.md 3.1..3.4; docs/design/domain.md 3.3, 5.2..5.3.
// This is a finite structural abstraction, NOT the complete GATES implementation.
// Cryptographic verification, HTTP outcomes, time, revocation, retry and
// configuration changes are outside this model.
// All Caller atoms represent already-verified GitHub OIDC identities.

sig Owner {}
sig OwnerName {}
sig RepoName { writtenOwner: one OwnerName }
sig Repository { actualOwner: one Owner, fullName: one RepoName }

abstract sig Permission {}
one sig ContentsRead, MetadataRead, ContentsWrite extends Permission {}
sig OtherPermission extends Permission {}

abstract sig InstallationStatus {}
one sig Active, Suspended extends InstallationStatus {}

abstract sig RepositorySelection {}
one sig Selected, AllRepositories extends RepositorySelection {}

abstract sig Answer {}
one sig Yes, No extends Answer {}

sig Caller { signedOwner: one Owner, sourceRepository: one Repository }

sig Installation {
  installedFor: one Owner,
  status: one InstallationStatus,
  coveredRepositories: set Repository,
  installedPermissions: set Permission
}

one sig Config {
  allowedOwner: one Owner,
  allowedName: one OwnerName,
  selectedInstallation: one Installation
}

sig TokenRequest {
  caller: one Caller,
  requestedNames: some RepoName,
  requestedPermissions: some Permission
}

sig TokenGrant {
  request: one TokenRequest,
  installation: one Installation,
  names: some RepoName,
  permissions: some Permission
}

sig IssueAttempt {
  grant: one TokenGrant,
  sentNames: some RepoName,
  sentPermissions: some Permission,
  selection: one RepositorySelection
}

// One element of the response's repository list. Separate atoms can refer to
// the same repository, so duplicates are representable. A Repository atom
// stands for one Repository ID.
sig ResponseEntry { repository: one Repository }

sig IssuanceResponse {
  attempt: one IssueAttempt,
  nonemptyToken: one Answer,
  futureExpiry: one Answer,
  entries: set ResponseEntry,
  effectivePermissions: set Permission,
  selection: one RepositorySelection
}

fun effectiveRepositories[r: IssuanceResponse]: set Repository {
  r.entries.repository
}

sig ReturnedToken { response: one IssuanceResponse }

// The baseline policy is used by the original checks and witnesses.
// The other policies deliberately disable exactly one return-time check.
// They are counterfactual verification models, not GATES configurations.
abstract sig VerificationPolicy {}
one sig Baseline, WithoutOwnerCheck, WithoutNameCheck,
        WithoutPermissionCheck, WithoutCountCheck extends VerificationPolicy {}
one sig ModelConfiguration { policy: one VerificationPolicy }

pred baseline { ModelConfiguration.policy = Baseline }

// A single GitHub issuance API invocation can yield at most one modeled
// response, and one response can result in at most one returned token.
fact ResponseCardinality {
  all a: IssueAttempt | lone a.~attempt
  all r: IssuanceResponse | lone r.~response
}

// Identity consistency comes from successful OIDC verification; it is not
// checked again by authorization. The source repository is NOT an ACL key.
fact VerifiedCallerSource {
  all c: Caller | c.sourceRepository.actualOwner = c.signedOwner
}

// The owner ID, written owner name and installation are fixed configuration;
// the owner name is modeled separately from the immutable owner ID.
pred authorize[r: TokenRequest, i: Installation] {
  r.caller.signedOwner = Config.allowedOwner
  all n: r.requestedNames | n.writtenOwner = Config.allowedName
  r.requestedPermissions = ContentsRead
  i = Config.selectedInstallation
  i.installedFor = Config.allowedOwner
  i.status = Active
}

// Represents the checks before the GitHub issuance API call. In particular,
// actual repo ownership / installation coverage is NOT a precondition here.
fact GrantsRespectAuthorization {
  all g: TokenGrant |
    authorize[g.request, g.installation] and
    g.names = g.request.requestedNames and
    g.permissions = g.request.requestedPermissions
}

// The API call always specifies exact requested names and permissions;
// it never inherits all repositories or all permissions by omission.
fact ExplicitIssueAttempts {
  all a: IssueAttempt |
    a.sentNames = a.grant.names and
    a.sentPermissions = a.grant.permissions and
    a.selection = Selected
}

// Represents return-time checks, NOT a guarantee about the actual GitHub
// token's authorization outside a trustworthy GitHub response.
pred verifiedResponse[r: IssuanceResponse] {
  r.nonemptyToken = Yes
  r.futureExpiry = Yes
  r.selection = Selected
  ContentsRead in r.effectivePermissions

  // Equal count and equal name set leave no room for a repeated name or ID.
  (ModelConfiguration.policy != WithoutCountCheck) implies
    #(r.entries) = #(r.attempt.grant.names)
  (ModelConfiguration.policy != WithoutNameCheck) implies
    effectiveRepositories[r].fullName = r.attempt.grant.names
  (ModelConfiguration.policy != WithoutOwnerCheck) implies
    (all x: effectiveRepositories[r] | x.actualOwner = Config.allowedOwner)
  (ModelConfiguration.policy != WithoutPermissionCheck) implies
    r.effectivePermissions in (ContentsRead + MetadataRead)
}

fact OnlyVerifiedTokensAreReturned {
  all t: ReturnedToken | verifiedResponse[t.response]
}

// The following is an *external GitHub behavior assumption*, deliberately
// not a fact: GitHub must enforce installation coverage and app permissions.
// GATES does not prevalidate the requested names against coverage.
pred githubResponseRespectsInstallation[r: IssuanceResponse] {
  effectiveRepositories[r] in r.attempt.grant.installation.coveredRepositories
  r.effectivePermissions in (r.attempt.grant.installation.installedPermissions + MetadataRead)
}

pred entriesAreUnique[r: IssuanceResponse] {
  all disj x, y: r.entries |
    x.repository != y.repository and x.repository.fullName != y.repository.fullName
}

assert NoForeignOwnerGrant {
  all g: TokenGrant | g.request.caller.signedOwner = Config.allowedOwner
}
assert NoOtherInstallationGrant {
  all g: TokenGrant | g.installation = Config.selectedInstallation and
    g.installation.installedFor = Config.allowedOwner and g.installation.status = Active
}
assert OnlyCurrentPermissionGranted {
  all g: TokenGrant | g.permissions = ContentsRead
}
assert AllOrNothingGrant {
  all g: TokenGrant | g.names = g.request.requestedNames
}
assert AllIssueAttemptsExplicit {
  all a: IssueAttempt |
    a.sentNames = a.grant.request.requestedNames and
    a.sentPermissions = ContentsRead and a.selection = Selected
}
assert NoOverbroadTokenReturned {
  baseline implies (all t: ReturnedToken |
    t.response.effectivePermissions in (ContentsRead + MetadataRead) and
    ContentsRead in t.response.effectivePermissions and
    t.response.selection = Selected)
}
assert ExactNamesReturned {
  baseline implies (all t: ReturnedToken |
    effectiveRepositories[t.response].fullName = t.response.attempt.grant.request.requestedNames and
    #(t.response.entries) = #(t.response.attempt.grant.request.requestedNames))
}
assert NoDuplicateEntriesReturned {
  baseline implies (all t: ReturnedToken | entriesAreUnique[t.response])
}
assert ReturnedReposOwnedByAllowedOwner {
  baseline implies (all t: ReturnedToken |
    all r: effectiveRepositories[t.response] | r.actualOwner = Config.allowedOwner)
}
// Conditional on the external assumption: GATES does not prevalidate coverage,
// and its full-name check rejects a response that omits an uncovered name.
assert CoverageHoldsWhenGitHubEnforces {
  baseline implies (all t: ReturnedToken |
    githubResponseRespectsInstallation[t.response] implies
    (all n: t.response.attempt.grant.names |
      some r: t.response.attempt.grant.installation.coveredRepositories |
        r.fullName = n and r.actualOwner = Config.allowedOwner))
}

// Mutation checks intentionally evaluate the same returned-token properties
// under weaker verification policies. Their results must be examined as
// counterexamples; they are not assertions about the baseline implementation.
assert WithoutOwnerCheckPreservesOwnerBoundary {
  ModelConfiguration.policy = WithoutOwnerCheck implies
    (all t: ReturnedToken |
      all r: effectiveRepositories[t.response] | r.actualOwner = Config.allowedOwner)
}
assert WithoutNameCheckPreservesExactNames {
  ModelConfiguration.policy = WithoutNameCheck implies
    (all t: ReturnedToken |
      effectiveRepositories[t.response].fullName = t.response.attempt.grant.request.requestedNames and
      #(t.response.entries) = #(t.response.attempt.grant.request.requestedNames))
}
assert WithoutCountCheckPreservesUniqueEntries {
  ModelConfiguration.policy = WithoutCountCheck implies
    (all t: ReturnedToken | entriesAreUnique[t.response])
}
assert WithoutPermissionCheckPreservesPermissionBoundary {
  ModelConfiguration.policy = WithoutPermissionCheck implies
    (all t: ReturnedToken |
      t.response.effectivePermissions in (ContentsRead + MetadataRead) and
      ContentsRead in t.response.effectivePermissions and
      t.response.selection = Selected)
}
assert ReturnedResultsHaveValidTokenAndExpiry {
  all t: ReturnedToken |
    t.response.nonemptyToken = Yes and t.response.futureExpiry = Yes
}

// Positive witness: successful issuance must be satisfiable, or assertions
// could pass vacuously in an inconsistent model.
pred SuccessfulReturn {
  baseline
  some t: ReturnedToken |
    githubResponseRespectsInstallation[t.response]
}

// Intended residual risk: a caller's source repository is DIFFERENT from the
// requested target, but both are within the same allowed owner. No per-caller
// access control is modeled, by design for Current Scope.
pred CrossRepositoryWithinOwner {
  baseline
  some t: ReturnedToken, target: Repository |
    githubResponseRespectsInstallation[t.response] and
    target in effectiveRepositories[t.response] and
    target != t.response.attempt.grant.request.caller.sourceRepository and
    target.fullName != t.response.attempt.grant.request.caller.sourceRepository.fullName
}

// Exploration of the trust boundary: without assuming GitHub validates the
// installation's coverage, response fields could appear self-consistent to
// GATES even when coverage is not proved by GATES. Such a witness does not
// establish exploitable GitHub API behavior.
pred AttemptMayContainUncoveredNames {
  baseline
  some a: IssueAttempt |
    some n: a.sentNames |
      no r: a.grant.installation.coveredRepositories | r.fullName = n
}

pred CoverageNotIndependentlyProved {
  baseline
  some t: ReturnedToken |
    effectiveRepositories[t.response] not in t.response.attempt.grant.installation.coveredRepositories
}

// Witness that an invalid response can exist independently of return.
// Non-return of overbroad responses is checked by NoOverbroadTokenReturned.
pred InvalidResponseCanExist {
  baseline
  some r: IssuanceResponse |
    ContentsWrite in r.effectivePermissions and
    no t: ReturnedToken | t.response = r
}

// All commands use a finite scope; absence of counterexamples is NOT
// an unbounded proof.
// Permission is 4 so that one unknown permission exists besides the three named ones.
check NoForeignOwnerGrant for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 Permission expect 0
check NoOtherInstallationGrant for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 Permission expect 0
check OnlyCurrentPermissionGranted for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 Permission expect 0
check AllOrNothingGrant for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 Permission expect 0
check AllIssueAttemptsExplicit for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 Permission expect 0
check NoOverbroadTokenReturned for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 Permission expect 0
check ExactNamesReturned for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 Permission expect 0
check NoDuplicateEntriesReturned for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 Permission expect 0
check ReturnedReposOwnedByAllowedOwner for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 Permission expect 0
check CoverageHoldsWhenGitHubEnforces for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 Permission expect 0
check ReturnedResultsHaveValidTokenAndExpiry for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 Permission expect 0

// These checks use a different verification policy from the baseline.
// A counterexample is expected: it shows that the removed check matters.
check WithoutOwnerCheckPreservesOwnerBoundary for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 Permission expect 1
check WithoutNameCheckPreservesExactNames for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 Permission expect 1
check WithoutPermissionCheckPreservesPermissionBoundary for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 Permission expect 1
check WithoutCountCheckPreservesUniqueEntries for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 Permission expect 1

run SuccessfulReturn for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 Permission expect 1
run CrossRepositoryWithinOwner for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 Permission expect 1
run AttemptMayContainUncoveredNames for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 Permission expect 1
run CoverageNotIndependentlyProved for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 Permission expect 1
run InvalidResponseCanExist for 3 but 5 VerificationPolicy, 2 Owner, 2 OwnerName, 4 Permission expect 1
