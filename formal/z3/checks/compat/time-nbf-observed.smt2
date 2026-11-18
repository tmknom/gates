; expect: unsat
; trace:checks GOJ-TIME-NBF GH-OIDC-TIME-OBSERVED
; In every issuance the Observed clauses allow, nbf is 300 seconds before
; iat, so GATES accepts its nbf at every now at which it accepts its iat,
; whatever the clocks.
(assert observed-issuance)
(assert (gates-iat issued))
(assert (gates-nbf issued))
(assert (gates-time-iat issued now))
(assert (not (gates-time-nbf issued now)))
(check-sat)
