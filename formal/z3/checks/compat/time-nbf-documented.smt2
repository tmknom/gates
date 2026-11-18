; expect: unsat
; trace:checks GOJ-TIME-NBF GH-OIDC-REGISTERED-CLAIMS
; GATES accepts, at now, every issuance the Documented clauses allow for use
; at usedAt, at or after its nbf, when the GitHub clock is at most L ahead of
; the Lambda clock. An integer nbf is the one GOJ-NBF admits.
(assert documented-issuance)
(assert clock-skew-within-leeway)
(assert (gates-nbf issued))
(assert (not (gates-time-nbf issued now)))
(check-sat)
