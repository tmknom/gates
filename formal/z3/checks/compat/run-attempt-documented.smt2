; expect: unsat
; trace:checks GOJ-RUN-ATTEMPT GH-OIDC-RUN-ATTEMPT
; GH-OIDC-RUN-ATTEMPT gives the meaning of run_attempt without its value.
; GATES accepts every run_attempt that is present (GH-OIDC-CLAIM-PRESENCE), a
; string (GH-OIDC-CLAIM-TYPE), not "0" (GH-OIDC-RUN-ATTEMPT-VALUE) and spelled
; as GOJ-ID-FORMAT asks (GH-OIDC-CLAIM-ID-FORMAT).
(assert documented-issuance)
(assert (string-member issued "run_attempt"))
(assert (not (= (string-member-value issued "run_attempt") "0")))
(assert (gates-id-text (string-member-value issued "run_attempt")))
(assert (not (gates-run-attempt issued)))
(check-sat)
