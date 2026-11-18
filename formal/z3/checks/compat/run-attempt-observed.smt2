; expect: unsat
; trace:checks GOJ-RUN-ATTEMPT GH-OIDC-RUN-ATTEMPT-OBSERVED
; In every issuance the Observed clauses allow, run_attempt is "1", which
; GATES accepts with no further assumption.
(assert observed-issuance)
(assert (not (gates-run-attempt issued)))
(check-sat)
