module gates/issuance_temporal

// GATES Current Scope: abstract one GitHub issuance API call per Lambda invocation.
// This intentionally models TWO invocations so client-controlled retry and
// duplicate remote issuance can be represented. No idempotency guarantee.
// Input: FR-007, FR-018, QR-003, QR-007, AC-12, AC-30, INV-012, INV-014;
// docs/design/security.md 3.4;
// docs/design/lambda.md 3.1 and 6.3.

abstract sig Phase {}
one sig Dormant, Start, Granted, IssueRequested, Returnable, NeedRevoke,
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
abstract sig Flag {}
one sig GrantOk, IssueCallSent, Issued, TokenKnown, ResultVerified, Delivered,
        SuccessObserved, RevokeTried, RevokeConfirmed, ResponseLost,
        InvalidWithoutToken extends Flag {}

abstract sig Invocation {}
one sig Original, Retry extends Invocation {}

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

pred GrantAllow[i: Invocation] {
  phaseOf[i] = Start
  advance[i, Granted, GrantOk]
}

pred GrantDeny[i: Invocation] {
  phaseOf[i] = Start
  advance[i, Failed, none]
}

pred CallIssueApi[i: Invocation] {
  phaseOf[i] = Granted and has[i, GrantOk]
  advance[i, IssueRequested, IssueCallSent]
}

pred ReceivedValid[i: Invocation] {
  phaseOf[i] = IssueRequested
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
  phaseOf[i] in (Start + Granted + IssueRequested + Returnable + NeedRevoke + Revoking)
  advance[i, Interrupted, none]
}

pred InterruptAfterIssue[i: Invocation] {
  phaseOf[i] = IssueRequested
  advance[i, Interrupted, Issued]
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
    StartRetry[i] or
    ObserveSuccess[i]
  )
}

fact Execution { Init and always Next }

// Safety properties within this abstraction, not statements about GitHub's
// actual access enforcement, physical execution or post-return freshness.
assert NoUnverifiedReturn {
  always (all i: Invocation |
    has[i, Delivered] implies
    (has[i, ResultVerified] and has[i, TokenKnown] and
     has[i, GrantOk] and has[i, IssueCallSent]))
}
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

// Executable witness predicates; evaluate their outcomes in Alloy Analyzer.
pred SuccessfulIssueWitness {
  eventually phaseOf[Original] = Succeeded
}
pred ObservedSuccessWitness {
  eventually has[Original, SuccessObserved]
}
pred RevocationConfirmedWitness {
  eventually (phaseOf[Original] = Failed and has[Original, RevokeConfirmed])
}
pred ResponseLossOrphanWitness {
  eventually (phaseOf[Original] = Failed and
    has[Original, Issued] and not has[Original, TokenKnown] and
    has[Original, ResponseLost])
}
pred RevokeFailureWitness {
  eventually (phaseOf[Original] = Failed and
    has[Original, RevokeTried] and not has[Original, RevokeConfirmed])
}
pred InterruptedBeforeRevokeWitness {
  eventually (phaseOf[Original] = Interrupted and
    has[Original, TokenKnown] and not has[Original, ResultVerified] and
    not has[Original, RevokeTried])
}
pred DuplicateIssueAfterRetryWitness {
  eventually (has[Original, ResponseLost] and
    has[Original, Issued] and has[Retry, Issued])
}

// Distinguish issuance from non-issuance when an invalid response has no token.
pred InvalidWithoutTokenIssuedWitness {
  eventually (phaseOf[Original] = Failed and
    has[Original, Issued] and not has[Original, TokenKnown] and
    not has[Original, ResponseLost] and
    has[Original, InvalidWithoutToken])
}
pred InvalidWithoutTokenNotIssuedWitness {
  eventually (phaseOf[Original] = Failed and
    not has[Original, Issued] and not has[Original, TokenKnown] and
    not has[Original, ResponseLost] and has[Original, IssueCallSent] and
    has[Original, InvalidWithoutToken])
}

// An original Invoke can complete successfully before its client observes
// the response; a caller-initiated retry may then issue a second token.
pred RetryAfterUnobservedSuccessWitness {
  eventually (phaseOf[Original] = Succeeded and
    not has[Original, SuccessObserved] and phaseOf[Retry] = Dormant and
    after (phaseOf[Retry] = Start and eventually has[Retry, Issued]))
}

// Both Invokes may be in progress: a retry can start while Original is IssueRequested.
pred ConcurrentRetryDuplicateIssueWitness {
  eventually (phaseOf[Original] = IssueRequested and phaseOf[Retry] = Start and
    eventually (has[Original, Issued] and has[Retry, Issued]))
}

// A retry can overtake an original that has not yet reached GitHub.
pred RetryCallsBeforeOriginalWitness {
  eventually (has[Retry, IssueCallSent] and not has[Original, IssueCallSent])
}

check NoUnverifiedReturn for 14 steps expect 0
check NoIssueBeforeExplicitGrant for 14 steps expect 0
check NoReturnAfterLostResponse for 14 steps expect 0
check NoRevokeWithoutKnownToken for 14 steps expect 0
check NoFailureWithoutRevokeAttempt for 14 steps expect 0
check NoConfirmedRevocationWithoutAttempt for 14 steps expect 0
check NoInterruptedInvocationReturns for 14 steps expect 0
check NoObservedSuccessWithoutReturn for 14 steps expect 0
run SuccessfulIssueWitness for 14 steps expect 1
run ObservedSuccessWitness for 14 steps expect 1
run RevocationConfirmedWitness for 14 steps expect 1
run ResponseLossOrphanWitness for 14 steps expect 1
run RevokeFailureWitness for 14 steps expect 1
run InterruptedBeforeRevokeWitness for 14 steps expect 1
run DuplicateIssueAfterRetryWitness for 14 steps expect 1
run InvalidWithoutTokenIssuedWitness for 14 steps expect 1
run InvalidWithoutTokenNotIssuedWitness for 14 steps expect 1
run RetryAfterUnobservedSuccessWitness for 14 steps expect 1
run ConcurrentRetryDuplicateIssueWitness for 14 steps expect 1
run RetryCallsBeforeOriginalWitness for 14 steps expect 1
