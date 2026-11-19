; expect: unsat
; trace:checks GOJ-NBF GH-OIDC-CLAIM-TYPE-OBSERVED
; GATES accepts the nbf of every issuance the Observed clauses allow as well,
; once nbf is an integer (GH-OIDC-CLAIM-TYPE) and the clocks are within its
; range: GH-OIDC-TIME-OBSERVED keeps nbf within 300 seconds of the time of use.
(assert observed-issuance)
(assert clocks-in-range)
(assert ((_ is JInteger) (member-value (select issued "nbf"))))
(assert (not (gates-nbf issued)))
(check-sat)
