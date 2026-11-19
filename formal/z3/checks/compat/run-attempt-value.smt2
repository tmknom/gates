; expect: sat
; trace:checks GOJ-RUN-ATTEMPT GH-OIDC-RUN-ATTEMPT-VALUE
; Nothing fixes the value of the first attempt, so an issuance whose
; run_attempt is "0" is allowed, which GATES rejects.
(assert documented-issuance)
(assert (= (select issued "run_attempt") (Present (JString "0"))))
(assert (not (gates-run-attempt issued)))
(check-sat)
