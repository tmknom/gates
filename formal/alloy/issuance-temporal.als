module gates/issuance_temporal

// GATES Current Scope: abstract one GitHub issuance API call per Lambda invocation.
// This intentionally models TWO invocations so client-controlled retry and
// duplicate remote issuance can be represented. No idempotency guarantee.
// With Repository Policy Enforcement, an invocation first issues a Policy Read
// Token, reads the Repository Policies with it, and tries to revoke it, all
// before the Token Grant.
// Input: FR-ISSUANCE-002, FR-ISSUANCE-004, FR-ISSUANCE-007, FR-AUTHZ-011,
// FR-AUTHZ-012, QR-003, AC-ISSUANCE-004, AC-ISSUANCE-006, AC-ISSUANCE-011,
// AC-ISSUANCE-012, INV-012, INV-014;
// docs/design/security.md 3.4 and 3.7;
// docs/design/lambda.md 3.1 and 6.3.

abstract sig Phase {}
one sig Dormant, Start, PolicyReadGranted, PolicyReadRequested, PolicyTokenHeld,
        PolicyRevokePending, PolicyRevoking, PoliciesRead,
        Granted, IssueRequested, Returnable, NeedRevoke,
        Revoking, Succeeded, Failed, Interrupted extends Phase {}

// Facts about an invocation. Once recorded, a flag is never cleared.
// `Issued` is ground-truth remote GitHub state, not necessarily observed by GATES.
// `TokenKnown` records whether GATES acquired a usable token body.
// `ResultVerified` means all issuance result checks passed, not just a 2xx status.
// `Delivered` means Lambda produced a successful return; it does NOT prove
// that the invoker received it. `SuccessObserved` models that separate event.
// The client may retry without having observed success.
// `ResponseLost` is the loss of the GitHub API response to Lambda, not of
// Lambda's response to the invoker.
// The `Policy*` flags are the same facts for the Policy Read Token.
// `PoliciesFetched` means every requested Repository Policy was read, valid
// or not; `PolicyFetchFailed` means a read failed transiently.
// `GrantOk` stands for every condition of the Token Grant, RULE-016 included.
// `TargetRecreated` is an event outside GATES: a target is deleted and
// re-created under the same name after its Repository Policy was read.
// `PolicyTokenDelivered` records that the returned token was the Policy Read
// Token; only a variant can set it.
abstract sig Flag {}
one sig GrantOk, IssueCallSent, Issued, TokenKnown, ResultVerified, Delivered,
        SuccessObserved, RevokeTried, RevokeConfirmed, ResponseLost,
        InvalidWithoutToken,
        PolicyReadGrantOk, PolicyReadCallSent, PolicyTokenIssued, PolicyTokenKnown,
        PolicyTokenVerified, PoliciesFetched, PolicyFetchFailed,
        PolicyRevokeTried, PolicyRevokeConfirmed, TargetRecreated,
        PolicyTokenDelivered extends Flag {}

abstract sig Invocation {}
one sig Original, Retry extends Invocation {}

// Repository Policy Enforcement, fixed for the deployment.
abstract sig Enforcement {}
one sig Enforced, NotEnforced extends Enforcement {}

// The baseline is the specified process. Each other variant breaks one
// ordering rule of the Repository Policy path, to show that a check does not
// pass vacuously. They are counterfactual models, not GATES behavior.
abstract sig Variant {}
one sig Baseline, GrantWithoutPolicy, PolicyReadWithoutGrant, PolicyReadWhenNotEnforced,
        SkipPolicyRevoke, FetchFailureAllows, WithoutRepositoryIdMatch,
        ReturnPolicyToken, FetchWithUnverifiedToken extends Variant {}

one sig Deployment { enforcement: one Enforcement, variant: one Variant }

pred enforced { Deployment.enforcement = Enforced }
pred baseline { Deployment.variant = Baseline }
pred variant[v: Variant] { Deployment.variant = v }

// Each Invocation is one Lambda invocation. A retry is NOT the same token.
one sig State {
  var phase: Invocation -> one Phase,
  var flags: Invocation -> Flag
}

fun phaseOf[i: Invocation]: Phase { i.(State.phase) }

pred has[i: Invocation, f: Flag] { f in i.(State.flags) }

// Moves one invocation to phase `p` and records `added`; nothing else changes.
pred advance[i: Invocation, p: Phase, added: set Flag] {
  State.phase' = State.phase ++ (i -> p)
  State.flags' = State.flags + (i -> added)
}

pred Init {
  phaseOf[Original] = Start
  phaseOf[Retry] = Dormant
  no State.flags
}

// The Token Grant. Without enforcement it follows the pre-authorization and
// the Installation checks. With enforcement it follows the reads of every
// Repository Policy and the revocation attempt of the Policy Read Token.
pred GrantAllow[i: Invocation] {
  (phaseOf[i] = Start and (not enforced or variant[GrantWithoutPolicy])) or
  (phaseOf[i] = PoliciesRead and
    (has[i, PoliciesFetched] or (variant[FetchFailureAllows] and has[i, PolicyFetchFailed])))
  advance[i, Granted, GrantOk]
}

// A rejection by the pre-authorization, the Installation checks, or RULE-016,
// and the end of an invocation whose Repository Policy read failed.
pred GrantDeny[i: Invocation] {
  phaseOf[i] in Start + PoliciesRead
  advance[i, Failed, none]
}

// The Policy Read Grant: every condition of the Token Grant except RULE-016.
pred PolicyReadGrantAllow[i: Invocation] {
  phaseOf[i] = Start and (enforced or variant[PolicyReadWhenNotEnforced])
  advance[i, PolicyReadGranted, PolicyReadGrantOk]
}

pred CallPolicyReadApi[i: Invocation] {
  (phaseOf[i] = PolicyReadGranted and has[i, PolicyReadGrantOk]) or
  (variant[PolicyReadWithoutGrant] and phaseOf[i] = Start and enforced)
  advance[i, PolicyReadRequested, PolicyReadCallSent]
}

pred PolicyReadValid[i: Invocation] {
  phaseOf[i] = PolicyReadRequested
  advance[i, PolicyTokenHeld, PolicyTokenIssued + PolicyTokenKnown + PolicyTokenVerified]
}

pred PolicyReadInvalidKnown[i: Invocation] {
  phaseOf[i] = PolicyReadRequested
  advance[i, PolicyRevokePending, PolicyTokenIssued + PolicyTokenKnown]
}

// A response lost or without a usable token, with or without a remote token,
// and a rejection by GitHub.
pred PolicyReadFailedIssued[i: Invocation] {
  phaseOf[i] = PolicyReadRequested
  advance[i, Failed, PolicyTokenIssued]
}

pred PolicyReadFailedNotIssued[i: Invocation] {
  phaseOf[i] = PolicyReadRequested
  advance[i, Failed, none]
}

pred FetchPolicies[i: Invocation] {
  phaseOf[i] = PolicyTokenHeld
  advance[i, PolicyRevokePending, PoliciesFetched]
}

pred FetchPoliciesFailed[i: Invocation] {
  phaseOf[i] = PolicyTokenHeld
  advance[i, PolicyRevokePending, PolicyFetchFailed]
}

// Variant: a Policy Read Token that failed verification is used for the reads.
pred FetchPoliciesWithUnverifiedToken[i: Invocation] {
  variant[FetchWithUnverifiedToken] and phaseOf[i] = PolicyRevokePending and
  has[i, PolicyTokenKnown] and not has[i, PolicyTokenVerified] and
  not has[i, PoliciesFetched] and not has[i, PolicyFetchFailed]
  advance[i, PolicyRevokePending, PoliciesFetched]
}

// After the revocation attempt, a verified token goes on to the Token Grant
// and a token that failed verification ends the invocation.
fun afterPolicyRevoke[i: Invocation]: Phase {
  has[i, PolicyTokenVerified] => PoliciesRead else Failed
}

pred BeginPolicyRevoke[i: Invocation] {
  phaseOf[i] = PolicyRevokePending and has[i, PolicyTokenKnown]
  advance[i, PolicyRevoking, PolicyRevokeTried]
}

pred PolicyRevocationConfirmed[i: Invocation] {
  phaseOf[i] = PolicyRevoking
  advance[i, afterPolicyRevoke[i], PolicyRevokeConfirmed]
}

pred PolicyRevocationFailed[i: Invocation] {
  phaseOf[i] = PolicyRevoking
  advance[i, afterPolicyRevoke[i], none]
}

// Variant: the revocation of the Policy Read Token is skipped.
pred SkipPolicyRevocation[i: Invocation] {
  variant[SkipPolicyRevoke] and phaseOf[i] = PolicyRevokePending
  advance[i, afterPolicyRevoke[i], none]
}

// Variant: the Policy Read Token is returned as if it were the result.
pred DeliverPolicyToken[i: Invocation] {
  variant[ReturnPolicyToken] and phaseOf[i] = PolicyTokenHeld and has[i, PolicyTokenVerified]
  advance[i, Succeeded, Delivered + PolicyTokenDelivered]
}

// An event outside GATES: after the Repository Policies were read and before
// the issuance API call, a target is deleted and re-created under its name.
pred RecreateTarget[i: Invocation] {
  enforced and has[i, PoliciesFetched] and
  not has[i, IssueCallSent] and not has[i, TargetRecreated] and
  phaseOf[i] in PolicyRevokePending + PolicyRevoking + PoliciesRead + Granted
  advance[i, phaseOf[i], TargetRecreated]
}

pred CallIssueApi[i: Invocation] {
  phaseOf[i] = Granted and has[i, GrantOk]
  advance[i, IssueRequested, IssueCallSent]
}

// A valid response passes every check, the Repository ID check against the
// Policy Read Token included; a re-created target fails that check, so its
// response is invalid and goes through `ReceivedInvalidKnown`.
pred ReceivedValid[i: Invocation] {
  phaseOf[i] = IssueRequested
  not has[i, TargetRecreated] or variant[WithoutRepositoryIdMatch]
  advance[i, Returnable, Issued + TokenKnown + ResultVerified]
}

pred ReceivedInvalidKnown[i: Invocation] {
  phaseOf[i] = IssueRequested
  advance[i, NeedRevoke, Issued + TokenKnown]
}

// An invalid response contains no usable token body. Its remote issuance
// outcome is not observable in this abstraction: both cases are possible.
pred ReceivedInvalidWithoutTokenIssued[i: Invocation] {
  phaseOf[i] = IssueRequested
  advance[i, Failed, Issued + InvalidWithoutToken]
}

pred ReceivedInvalidWithoutTokenNotIssued[i: Invocation] {
  phaseOf[i] = IssueRequested
  advance[i, Failed, InvalidWithoutToken]
}

pred ResponseLostIssued[i: Invocation] {
  phaseOf[i] = IssueRequested
  advance[i, Failed, Issued + ResponseLost]
}

pred ResponseLostNotIssued[i: Invocation] {
  phaseOf[i] = IssueRequested
  advance[i, Failed, ResponseLost]
}

pred GitHubIssuanceRejected[i: Invocation] {
  phaseOf[i] = IssueRequested
  advance[i, Failed, none]
}

pred Deliver[i: Invocation] {
  phaseOf[i] = Returnable and has[i, TokenKnown] and has[i, ResultVerified]
  advance[i, Succeeded, Delivered]
}

pred BeginRevoke[i: Invocation] {
  phaseOf[i] = NeedRevoke and has[i, TokenKnown]
  advance[i, Revoking, RevokeTried]
}

pred RevocationConfirmed[i: Invocation] {
  phaseOf[i] = Revoking
  advance[i, Failed, RevokeConfirmed]
}

pred RevocationFailed[i: Invocation] {
  phaseOf[i] = Revoking
  advance[i, Failed, none]
}

pred Interrupt[i: Invocation] {
  phaseOf[i] in (Start + PolicyReadGranted + PolicyReadRequested + PolicyTokenHeld +
                 PolicyRevokePending + PolicyRevoking + PoliciesRead +
                 Granted + IssueRequested + Returnable + NeedRevoke + Revoking)
  advance[i, Interrupted, none]
}

pred InterruptAfterIssue[i: Invocation] {
  phaseOf[i] = IssueRequested
  advance[i, Interrupted, Issued]
}

pred InterruptAfterPolicyIssue[i: Invocation] {
  phaseOf[i] = PolicyReadRequested
  advance[i, Interrupted, PolicyTokenIssued]
}

// The retry is a new synchronous Invoke initiated by the caller, not an
// automatic Lambda retry. Both unobserved success and an overlapping invocation
// are included as abstract possibilities, not verified AWS caller behavior.
// Overlap is possible at any point while the original invocation is progressing;
// a completed success is retryable only if the client has not observed it.
// This covers retries after a lost response or timeout, not repeated issuance
// in general: GATES does not stop a caller from requesting another token.
pred StartRetry[i: Invocation] {
  i = Retry and phaseOf[Retry] = Dormant
  phaseOf[Original] = Succeeded implies not has[Original, SuccessObserved]
  advance[i, Start, none]
}

// A separate observation step captures successful delivery to the caller.
// Absence of this transition allows a completed Lambda invocation whose
// successful response was not observed. No transport behavior is guaranteed.
pred ObserveSuccess[i: Invocation] {
  phaseOf[i] = Succeeded and not has[i, SuccessObserved]
  advance[i, Succeeded, SuccessObserved]
}

pred Stutter {
  State.phase' = State.phase
  State.flags' = State.flags
}

pred Next {
  Stutter or
  (some i: Invocation |
    GrantAllow[i] or
    GrantDeny[i] or
    PolicyReadGrantAllow[i] or
    CallPolicyReadApi[i] or
    PolicyReadValid[i] or
    PolicyReadInvalidKnown[i] or
    PolicyReadFailedIssued[i] or
    PolicyReadFailedNotIssued[i] or
    FetchPolicies[i] or
    FetchPoliciesFailed[i] or
    FetchPoliciesWithUnverifiedToken[i] or
    BeginPolicyRevoke[i] or
    PolicyRevocationConfirmed[i] or
    PolicyRevocationFailed[i] or
    SkipPolicyRevocation[i] or
    DeliverPolicyToken[i] or
    RecreateTarget[i] or
    CallIssueApi[i] or
    ReceivedValid[i] or
    ReceivedInvalidKnown[i] or
    ReceivedInvalidWithoutTokenIssued[i] or
    ReceivedInvalidWithoutTokenNotIssued[i] or
    ResponseLostIssued[i] or
    ResponseLostNotIssued[i] or
    GitHubIssuanceRejected[i] or
    Deliver[i] or
    BeginRevoke[i] or
    RevocationConfirmed[i] or
    RevocationFailed[i] or
    Interrupt[i] or
    InterruptAfterIssue[i] or
    InterruptAfterPolicyIssue[i] or
    StartRetry[i] or
    ObserveSuccess[i]
  )
}

fact Execution { Init and always Next }

// Safety properties within this abstraction, not statements about GitHub's
// actual access enforcement, physical execution or post-return freshness.
// The variant returning the Policy Read Token breaks this property too, so it
// is stated for every other variant.
assert NoUnverifiedReturn {
  not variant[ReturnPolicyToken] implies always (all i: Invocation |
    has[i, Delivered] implies
    (has[i, ResultVerified] and has[i, TokenKnown] and
     has[i, GrantOk] and has[i, IssueCallSent]))
}
// `GrantOk` stands for every condition of the Token Grant, those of the
// protected repositories and the Repository Policies included. A renamed
// protected repository requested by its new name meets them, so GitHub may
// issue only after `GrantOk`; its result then fails verification, which
// `ReceivedInvalidKnown` covers.
assert NoIssueBeforeExplicitGrant {
  always (all i: Invocation |
    has[i, Issued] implies (has[i, GrantOk] and has[i, IssueCallSent]))
}
assert NoReturnAfterLostResponse {
  always (all i: Invocation |
    has[i, ResponseLost] implies not has[i, Delivered])
}
assert NoRevokeWithoutKnownToken {
  always (all i: Invocation |
    has[i, RevokeTried] implies has[i, TokenKnown])
}
// An interrupted invocation is excluded: see InterruptedBeforeRevokeWitness.
assert NoFailureWithoutRevokeAttempt {
  always (all i: Invocation |
    (phaseOf[i] = Failed and has[i, TokenKnown] and not has[i, ResultVerified])
    implies has[i, RevokeTried])
}
assert NoConfirmedRevocationWithoutAttempt {
  always (all i: Invocation |
    has[i, RevokeConfirmed] implies has[i, RevokeTried])
}
assert NoInterruptedInvocationReturns {
  always (all i: Invocation |
    phaseOf[i] = Interrupted implies not has[i, Delivered])
}

assert NoObservedSuccessWithoutReturn {
  always (all i: Invocation |
    has[i, SuccessObserved] implies
    (has[i, Delivered] and phaseOf[i] = Succeeded))
}

// The token returned to the user is requested only after a Token Grant that,
// with enforcement, followed the reads of every Repository Policy with a
// verified Policy Read Token.
pred userTokenAfterPolicyGrant {
  always (all i: Invocation |
    has[i, IssueCallSent] implies
      (has[i, GrantOk] and
       (enforced implies
         (has[i, PolicyTokenVerified] and has[i, PoliciesFetched] and not has[i, PolicyFetchFailed]))))
}
assert NoUserTokenBeforePolicyGrant {
  baseline implies userTokenAfterPolicyGrant
}

// A Policy Read Token is issued only with enforcement and after the Policy
// Read Grant: every condition of the Token Grant except RULE-016.
pred policyTokenAfterPolicyReadGrant {
  always (all i: Invocation |
    has[i, PolicyReadCallSent] implies (enforced and has[i, PolicyReadGrantOk]))
}
assert NoPolicyTokenBeforePolicyReadGrant {
  baseline implies policyTokenAfterPolicyReadGrant
}

// The returned token is never the Policy Read Token.
pred policyTokenNeverReturned {
  always (all i: Invocation | not has[i, PolicyTokenDelivered])
}
assert NoPolicyTokenReturned {
  baseline implies policyTokenNeverReturned
}

// A Policy Read Token whose body GATES got is revoked before the invocation
// ends, however it ends, except by an interruption.
pred policyTokenRevokedBeforeEnd {
  always (all i: Invocation |
    (phaseOf[i] in Failed + Succeeded and has[i, PolicyTokenKnown]) implies
      has[i, PolicyRevokeTried])
}
assert PolicyTokenRevokeAttempted {
  baseline implies policyTokenRevokedBeforeEnd
}

// A target re-created after its Repository Policy was read gets no returned
// token: the Repository ID check against the Policy Read Token fails.
pred noReturnForRecreatedTarget {
  always (all i: Invocation | has[i, Delivered] implies not has[i, TargetRecreated])
}
assert NoReturnForRecreatedTarget {
  baseline implies noReturnForRecreatedTarget
}

// A read of a Repository Policy that failed leads to no issuance and no
// return.
pred noIssueAfterPolicyFetchFailure {
  always (all i: Invocation |
    has[i, PolicyFetchFailed] implies (not has[i, IssueCallSent] and not has[i, Delivered]))
}
assert NoIssueAfterPolicyFetchFailure {
  baseline implies noIssueAfterPolicyFetchFailure
}

// Repository Policies are read only with a Policy Read Token that passed
// verification, whether the read succeeds or fails.
pred policiesReadOnlyWithVerifiedToken {
  always (all i: Invocation |
    (has[i, PoliciesFetched] or has[i, PolicyFetchFailed]) implies has[i, PolicyTokenVerified])
}
assert NoFetchWithUnverifiedPolicyToken {
  baseline implies policiesReadOnlyWithVerifiedToken
}

// Each variant breaks the property of the same rule.
assert GrantWithoutPolicyBreaksOrder {
  variant[GrantWithoutPolicy] implies userTokenAfterPolicyGrant
}
assert PolicyReadWithoutGrantBreaksOrder {
  variant[PolicyReadWithoutGrant] implies policyTokenAfterPolicyReadGrant
}
assert PolicyReadWhenNotEnforcedBreaksOrder {
  variant[PolicyReadWhenNotEnforced] implies policyTokenAfterPolicyReadGrant
}
assert ReturnPolicyTokenBreaksNonReturn {
  variant[ReturnPolicyToken] implies policyTokenNeverReturned
}
assert SkipPolicyRevokeBreaksRevocation {
  variant[SkipPolicyRevoke] implies policyTokenRevokedBeforeEnd
}
assert WithoutRepositoryIdMatchBreaksRecreation {
  variant[WithoutRepositoryIdMatch] implies noReturnForRecreatedTarget
}
assert FetchFailureAllowsBreaksFailure {
  variant[FetchFailureAllows] implies noIssueAfterPolicyFetchFailure
}
assert FetchWithUnverifiedTokenBreaksVerification {
  variant[FetchWithUnverifiedToken] implies policiesReadOnlyWithVerifiedToken
}

// Executable witness predicates; evaluate their outcomes in Alloy Analyzer.
pred SuccessfulIssueWitness {
  baseline
  eventually phaseOf[Original] = Succeeded
}
pred ObservedSuccessWitness {
  baseline
  eventually has[Original, SuccessObserved]
}
pred RevocationConfirmedWitness {
  baseline
  eventually (phaseOf[Original] = Failed and has[Original, RevokeConfirmed])
}
pred ResponseLossOrphanWitness {
  baseline
  eventually (phaseOf[Original] = Failed and
    has[Original, Issued] and not has[Original, TokenKnown] and
    has[Original, ResponseLost])
}
pred RevokeFailureWitness {
  baseline
  eventually (phaseOf[Original] = Failed and
    has[Original, RevokeTried] and not has[Original, RevokeConfirmed])
}
pred InterruptedBeforeRevokeWitness {
  baseline
  eventually (phaseOf[Original] = Interrupted and
    has[Original, TokenKnown] and not has[Original, ResultVerified] and
    not has[Original, RevokeTried])
}
pred DuplicateIssueAfterRetryWitness {
  baseline
  eventually (has[Original, ResponseLost] and
    has[Original, Issued] and has[Retry, Issued])
}

// Distinguish issuance from non-issuance when an invalid response has no token.
pred InvalidWithoutTokenIssuedWitness {
  baseline
  eventually (phaseOf[Original] = Failed and
    has[Original, Issued] and not has[Original, TokenKnown] and
    not has[Original, ResponseLost] and
    has[Original, InvalidWithoutToken])
}
pred InvalidWithoutTokenNotIssuedWitness {
  baseline
  eventually (phaseOf[Original] = Failed and
    not has[Original, Issued] and not has[Original, TokenKnown] and
    not has[Original, ResponseLost] and has[Original, IssueCallSent] and
    has[Original, InvalidWithoutToken])
}

// An original Invoke can complete successfully before its client observes
// the response; a caller-initiated retry may then issue a second token.
pred RetryAfterUnobservedSuccessWitness {
  baseline
  eventually (phaseOf[Original] = Succeeded and
    not has[Original, SuccessObserved] and phaseOf[Retry] = Dormant and
    after (phaseOf[Retry] = Start and eventually has[Retry, Issued]))
}

// Both Invokes may be in progress: a retry can start while Original is IssueRequested.
pred ConcurrentRetryDuplicateIssueWitness {
  baseline
  eventually (phaseOf[Original] = IssueRequested and phaseOf[Retry] = Start and
    eventually (has[Original, Issued] and has[Retry, Issued]))
}

// A retry can overtake an original that has not yet reached GitHub.
pred RetryCallsBeforeOriginalWitness {
  baseline
  eventually (has[Retry, IssueCallSent] and not has[Original, IssueCallSent])
}

// With enforcement, an invocation reads the Repository Policies, tries to
// revoke the Policy Read Token, and returns a token.
pred EnforcedSuccessWitness {
  baseline and enforced
  eventually (phaseOf[Original] = Succeeded and has[Original, PolicyRevokeTried])
}
// A failed revocation of the Policy Read Token does not change the result.
pred PolicyRevokeFailureStillSucceedsWitness {
  baseline and enforced
  eventually (phaseOf[Original] = Succeeded and
    has[Original, PolicyRevokeTried] and not has[Original, PolicyRevokeConfirmed])
}
// A failed read ends the invocation after the revocation attempt.
pred PolicyFetchFailureWitness {
  baseline and enforced
  eventually (phaseOf[Original] = Failed and
    has[Original, PolicyFetchFailed] and has[Original, PolicyRevokeTried])
}
// A Policy Read Token that fails verification is revoked and never used.
pred PolicyTokenVerificationFailureWitness {
  baseline and enforced
  eventually (phaseOf[Original] = Failed and has[Original, PolicyTokenKnown] and
    not has[Original, PolicyTokenVerified] and has[Original, PolicyRevokeTried] and
    not has[Original, PoliciesFetched])
}
// A target re-created after the read: GitHub issues the user token, which
// fails verification and is revoked.
pred RecreatedTargetRevokedWitness {
  baseline and enforced
  eventually (phaseOf[Original] = Failed and has[Original, TargetRecreated] and
    has[Original, Issued] and has[Original, RevokeTried])
}
// Accepted risk: an interruption before the revocation attempt leaves the
// Policy Read Token valid on GitHub.
pred InterruptedPolicyTokenLeftWitness {
  baseline and enforced
  eventually (phaseOf[Original] = Interrupted and
    has[Original, PolicyTokenKnown] and not has[Original, PolicyRevokeTried])
}
// A lost response to the Policy Read Token issuance leaves a token GATES
// never got.
pred PolicyTokenResponseLossWitness {
  baseline and enforced
  eventually (phaseOf[Original] = Failed and
    has[Original, PolicyTokenIssued] and not has[Original, PolicyTokenKnown])
}

// The longest invocation has 12 transitions with enforcement; the retry adds
// one to start, so 26 states cover every execution of both invocations.
check NoUnverifiedReturn for 26 steps expect 0
check NoIssueBeforeExplicitGrant for 26 steps expect 0
check NoReturnAfterLostResponse for 26 steps expect 0
check NoRevokeWithoutKnownToken for 26 steps expect 0
check NoFailureWithoutRevokeAttempt for 26 steps expect 0
check NoConfirmedRevocationWithoutAttempt for 26 steps expect 0
check NoInterruptedInvocationReturns for 26 steps expect 0
check NoObservedSuccessWithoutReturn for 26 steps expect 0
// trace:checks FR-ISSUANCE-002 FR-AUTHZ-011 INV-014
check NoUserTokenBeforePolicyGrant for 26 steps expect 0
// trace:checks FR-ISSUANCE-002 FR-ISSUANCE-007 INV-020
check NoPolicyTokenBeforePolicyReadGrant for 26 steps expect 0
// trace:checks FR-ISSUANCE-005 FR-ISSUANCE-007 INV-014
check NoPolicyTokenReturned for 26 steps expect 0
// trace:checks FR-ISSUANCE-007 INV-014 AC-ISSUANCE-011
check PolicyTokenRevokeAttempted for 26 steps expect 0
// trace:checks FR-AUTHZ-012 INV-012 AC-ISSUANCE-012
check NoReturnForRecreatedTarget for 26 steps expect 0
// trace:checks FR-AUTHZ-011 QR-003
check NoIssueAfterPolicyFetchFailure for 26 steps expect 0
// trace:checks FR-ISSUANCE-004 FR-ISSUANCE-007 AC-ISSUANCE-011
check NoFetchWithUnverifiedPolicyToken for 26 steps expect 0
// trace:checks FR-ISSUANCE-002
check GrantWithoutPolicyBreaksOrder for 26 steps expect 1
// trace:checks FR-ISSUANCE-002 FR-ISSUANCE-007
check PolicyReadWithoutGrantBreaksOrder for 26 steps expect 1
// trace:checks FR-ISSUANCE-007
check PolicyReadWhenNotEnforcedBreaksOrder for 26 steps expect 1
// trace:checks FR-ISSUANCE-007
check ReturnPolicyTokenBreaksNonReturn for 26 steps expect 1
// trace:checks FR-ISSUANCE-007
check SkipPolicyRevokeBreaksRevocation for 26 steps expect 1
// trace:checks FR-AUTHZ-012
check WithoutRepositoryIdMatchBreaksRecreation for 26 steps expect 1
// trace:checks FR-AUTHZ-011
check FetchFailureAllowsBreaksFailure for 26 steps expect 1
// trace:checks FR-ISSUANCE-004
check FetchWithUnverifiedTokenBreaksVerification for 26 steps expect 1
run SuccessfulIssueWitness for 26 steps expect 1
run ObservedSuccessWitness for 26 steps expect 1
run RevocationConfirmedWitness for 26 steps expect 1
run ResponseLossOrphanWitness for 26 steps expect 1
run RevokeFailureWitness for 26 steps expect 1
run InterruptedBeforeRevokeWitness for 26 steps expect 1
run DuplicateIssueAfterRetryWitness for 26 steps expect 1
run InvalidWithoutTokenIssuedWitness for 26 steps expect 1
run InvalidWithoutTokenNotIssuedWitness for 26 steps expect 1
run RetryAfterUnobservedSuccessWitness for 26 steps expect 1
run ConcurrentRetryDuplicateIssueWitness for 26 steps expect 1
run RetryCallsBeforeOriginalWitness for 26 steps expect 1
run EnforcedSuccessWitness for 26 steps expect 1
run PolicyRevokeFailureStillSucceedsWitness for 26 steps expect 1
run PolicyFetchFailureWitness for 26 steps expect 1
run PolicyTokenVerificationFailureWitness for 26 steps expect 1
run RecreatedTargetRevokedWitness for 26 steps expect 1
// trace:checks FR-ISSUANCE-007
run InterruptedPolicyTokenLeftWitness for 26 steps expect 1
run PolicyTokenResponseLossWitness for 26 steps expect 1
