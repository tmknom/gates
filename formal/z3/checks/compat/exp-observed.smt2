; expect: unsat
; trace:checks GOJ-EXP GH-OIDC-CLAIM-TYPE-OBSERVED
; GATES accepts the exp of every issuance the Observed clauses allow as well,
; once exp is an integer (GH-OIDC-CLAIM-TYPE) and the clocks are within its
; range: GH-OIDC-TIME-OBSERVED keeps exp within 300 seconds of the time of use.
(assert observed-issuance)
(assert clocks-in-range)
(assert ((_ is JInteger) (member-value (select issued "exp"))))
(assert (not (gates-exp issued)))
(check-sat)
