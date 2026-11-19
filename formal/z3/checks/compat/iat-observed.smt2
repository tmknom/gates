; expect: unsat
; trace:checks GOJ-IAT GH-OIDC-CLAIM-TYPE-OBSERVED
; GATES accepts the iat of every issuance the Observed clauses allow as well,
; once iat is an integer (GH-OIDC-CLAIM-TYPE) and the clocks are within its
; range: GH-OIDC-TIME-OBSERVED keeps iat within 300 seconds of the time of use.
(assert observed-issuance)
(assert clocks-in-range)
(assert ((_ is JInteger) (member-value (select issued "iat"))))
(assert (not (gates-iat issued)))
(check-sat)
