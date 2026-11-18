; expect: unsat
; trace:checks GOJ-TIME-IAT GH-OIDC-REGISTERED-CLAIMS
; GATES accepts, at now, every issuance the Documented clauses allow for use
; at usedAt, after its iat, when the GitHub clock is at most L ahead of the
; Lambda clock. An integer iat is the one GOJ-IAT admits.
(assert documented-issuance)
(assert clock-skew-within-leeway)
(assert (gates-iat issued))
(assert (not (gates-time-iat issued now)))
(check-sat)
