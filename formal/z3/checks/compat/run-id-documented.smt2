; expect: unsat
; trace:checks GOJ-RUN-ID GH-OIDC-RUN-ID GH-OIDC-RUN-ID-RERUN
; GH-OIDC-RUN-ID gives the meaning of run_id and GH-OIDC-RUN-ID-RERUN leaves
; open whether a re-run changes it; neither constrains the value. GATES
; accepts every run_id that is present (GH-OIDC-CLAIM-PRESENCE), a string
; (GH-OIDC-CLAIM-TYPE) and spelled as GOJ-ID-FORMAT asks
; (GH-OIDC-CLAIM-ID-FORMAT), whatever the run.
(assert documented-issuance)
(assert (string-member issued "run_id"))
(assert (gates-id-text (string-member-value issued "run_id")))
(assert (not (gates-run-id issued)))
(check-sat)
