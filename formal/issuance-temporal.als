module gates/issuance_temporal

// GATES Current Scope: abstract one GitHub issue request per Lambda invocation.
// This intentionally models TWO invocations so client-controlled retry and
// duplicate remote issuance can be represented. No idempotency guarantee.
// Input: docs/spec/requirements.md FR-007, FR-018, QR-003, QR-007, AC-12, AC-30;
// docs/design/security.md 3.4; docs/design/lambda.md 3.1 and 7.3.

abstract sig Phase {}
one sig Dormant, Start, Granted, Sent, Ready, NeedRevoke,
        Revoking, Succeeded, Failed, Interrupted extends Phase {}

// Facts about an attempt. Once recorded, a flag is never cleared.
// `Issued` is ground-truth remote GitHub state, not necessarily observed by GATES.
// `TokenKnown` records whether GATES acquired a usable token body.
// `ResponseVerified` means all issuance response checks passed, not just a 2xx status.
// `Delivered` means Lambda produced a successful return; it does NOT prove
// that the invoker received it. `SuccessObserved` models that separate event.
// The client may retry without having observed success.
// `ResponseLost` is the loss of the GitHub API response to Lambda, not of
// Lambda's response to the invoker.
abstract sig Flag {}
one sig GrantOk, RequestSent, Issued, TokenKnown, ResponseVerified, Delivered,
        SuccessObserved, RevokeTried, RevokeConfirmed, ResponseLost,
        InvalidWithoutToken extends Flag {}

abstract sig Attempt {}
one sig Original, Retry extends Attempt {}

// Each Attempt is one Lambda invocation. A retry is NOT the same token.
one sig State {
  var phase: Attempt -> one Phase,
  var flags: Attempt -> Flag
}

fun phaseOf[a: Attempt]: Phase { a.(State.phase) }

pred has[a: Attempt, f: Flag] { f in a.(State.flags) }

// Moves one attempt to phase `p` and records `added`; nothing else changes.
pred advance[a: Attempt, p: Phase, added: set Flag] {
  State.phase' = State.phase ++ (a -> p)
  State.flags' = State.flags + (a -> added)
}

pred Init {
  phaseOf[Original] = Start
  phaseOf[Retry] = Dormant
  no State.flags
}

pred GrantAllow[a: Attempt] {
  phaseOf[a] = Start
  advance[a, Granted, GrantOk]
}

pred GrantDeny[a: Attempt] {
  phaseOf[a] = Start
  advance[a, Failed, none]
}

pred Send[a: Attempt] {
  phaseOf[a] = Granted and has[a, GrantOk]
  advance[a, Sent, RequestSent]
}

pred ReceivedValid[a: Attempt] {
  phaseOf[a] = Sent
  advance[a, Ready, Issued + TokenKnown + ResponseVerified]
}

pred ReceivedInvalidKnown[a: Attempt] {
  phaseOf[a] = Sent
  advance[a, NeedRevoke, Issued + TokenKnown]
}

// An invalid response contains no usable token body. Its remote issuance
// outcome is not observable in this abstraction: both cases are possible.
pred ReceivedInvalidWithoutTokenIssued[a: Attempt] {
  phaseOf[a] = Sent
  advance[a, Failed, Issued + InvalidWithoutToken]
}

pred ReceivedInvalidWithoutTokenNotIssued[a: Attempt] {
  phaseOf[a] = Sent
  advance[a, Failed, InvalidWithoutToken]
}

pred ResponseLostIssued[a: Attempt] {
  phaseOf[a] = Sent
  advance[a, Failed, Issued + ResponseLost]
}

pred ResponseLostNotIssued[a: Attempt] {
  phaseOf[a] = Sent
  advance[a, Failed, ResponseLost]
}

pred ApiReject[a: Attempt] {
  phaseOf[a] = Sent
  advance[a, Failed, none]
}

pred Deliver[a: Attempt] {
  phaseOf[a] = Ready and has[a, TokenKnown] and has[a, ResponseVerified]
  advance[a, Succeeded, Delivered]
}

pred BeginRevoke[a: Attempt] {
  phaseOf[a] = NeedRevoke and has[a, TokenKnown]
  advance[a, Revoking, RevokeTried]
}

pred RevocationConfirmed[a: Attempt] {
  phaseOf[a] = Revoking
  advance[a, Failed, RevokeConfirmed]
}

pred RevocationFailed[a: Attempt] {
  phaseOf[a] = Revoking
  advance[a, Failed, none]
}

pred Interrupt[a: Attempt] {
  phaseOf[a] in (Start + Granted + Sent + Ready + NeedRevoke + Revoking)
  advance[a, Interrupted, none]
}

pred InterruptAfterIssue[a: Attempt] {
  phaseOf[a] = Sent
  advance[a, Interrupted, Issued]
}

// The retry is a new synchronous Invoke initiated by the caller, not an
// automatic Lambda retry. Both unobserved success and an overlapping attempt
// are included as abstract possibilities, not verified AWS caller behavior.
// Overlap is possible at any point while the original invocation is progressing;
// a completed success is retryable only if the client has not observed it.
// This covers retries after a lost response or timeout, not repeated issuance
// in general: GATES does not stop a caller from requesting another token.
pred StartRetry[a: Attempt] {
  a = Retry and phaseOf[Retry] = Dormant
  phaseOf[Original] = Succeeded implies not has[Original, SuccessObserved]
  advance[a, Start, none]
}

// A separate observation step captures successful delivery to the caller.
// Absence of this transition allows a completed Lambda invocation whose
// successful response was not observed. No transport behavior is guaranteed.
pred ObserveSuccess[a: Attempt] {
  phaseOf[a] = Succeeded and not has[a, SuccessObserved]
  advance[a, Succeeded, SuccessObserved]
}

pred Stutter {
  State.phase' = State.phase
  State.flags' = State.flags
}

pred Next {
  Stutter or
  (some a: Attempt |
    GrantAllow[a] or
    GrantDeny[a] or
    Send[a] or
    ReceivedValid[a] or
    ReceivedInvalidKnown[a] or
    ReceivedInvalidWithoutTokenIssued[a] or
    ReceivedInvalidWithoutTokenNotIssued[a] or
    ResponseLostIssued[a] or
    ResponseLostNotIssued[a] or
    ApiReject[a] or
    Deliver[a] or
    BeginRevoke[a] or
    RevocationConfirmed[a] or
    RevocationFailed[a] or
    Interrupt[a] or
    InterruptAfterIssue[a] or
    StartRetry[a] or
    ObserveSuccess[a]
  )
}

fact Execution { Init and always Next }

// Safety properties within this abstraction, not statements about GitHub's
// actual access enforcement, physical execution or post-return freshness.
assert NoUnverifiedReturn {
  always (all a: Attempt |
    has[a, Delivered] implies
    (has[a, ResponseVerified] and has[a, TokenKnown] and
     has[a, GrantOk] and has[a, RequestSent]))
}
assert NoIssueBeforeExplicitGrant {
  always (all a: Attempt |
    has[a, Issued] implies (has[a, GrantOk] and has[a, RequestSent]))
}
assert NoReturnAfterLostResponse {
  always (all a: Attempt |
    has[a, ResponseLost] implies not has[a, Delivered])
}
assert NoRevokeWithoutKnownToken {
  always (all a: Attempt |
    has[a, RevokeTried] implies has[a, TokenKnown])
}
// An interrupted attempt is excluded: see InterruptedBeforeRevokeWitness.
assert NoFailureWithoutRevokeAttempt {
  always (all a: Attempt |
    (phaseOf[a] = Failed and has[a, TokenKnown] and not has[a, ResponseVerified])
    implies has[a, RevokeTried])
}
assert NoConfirmedRevocationWithoutAttempt {
  always (all a: Attempt |
    has[a, RevokeConfirmed] implies has[a, RevokeTried])
}
assert NoAbortedAttemptReturns {
  always (all a: Attempt |
    phaseOf[a] = Interrupted implies not has[a, Delivered])
}

assert NoObservedSuccessWithoutReturn {
  always (all a: Attempt |
    has[a, SuccessObserved] implies
    (has[a, Delivered] and phaseOf[a] = Succeeded))
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
    has[Original, TokenKnown] and not has[Original, ResponseVerified] and
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
    not has[Original, ResponseLost] and has[Original, RequestSent] and
    has[Original, InvalidWithoutToken])
}

// An original Invoke can complete successfully before its client observes
// the response; a caller-initiated retry may then issue a second token.
pred RetryAfterUnobservedSuccessWitness {
  eventually (phaseOf[Original] = Succeeded and
    not has[Original, SuccessObserved] and phaseOf[Retry] = Dormant and
    after (phaseOf[Retry] = Start and eventually has[Retry, Issued]))
}

// Both Invokes may be in progress: a retry can start while Original is Sent.
pred ConcurrentRetryDuplicateIssueWitness {
  eventually (phaseOf[Original] = Sent and phaseOf[Retry] = Start and
    eventually (has[Original, Issued] and has[Retry, Issued]))
}

// A retry can overtake an original that has not yet reached GitHub.
pred RetrySentBeforeOriginalWitness {
  eventually (has[Retry, RequestSent] and not has[Original, RequestSent])
}

check NoUnverifiedReturn for 14 steps expect 0
check NoIssueBeforeExplicitGrant for 14 steps expect 0
check NoReturnAfterLostResponse for 14 steps expect 0
check NoRevokeWithoutKnownToken for 14 steps expect 0
check NoFailureWithoutRevokeAttempt for 14 steps expect 0
check NoConfirmedRevocationWithoutAttempt for 14 steps expect 0
check NoAbortedAttemptReturns for 14 steps expect 0
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
run RetrySentBeforeOriginalWitness for 14 steps expect 1
