; expect: sat
; trace:checks GOJ-IAT
; The Documented clauses bound iat by neither a type nor a range. Even for a
; present integer iat (GH-OIDC-CLAIM-PRESENCE, GH-OIDC-CLAIM-TYPE), they allow
; one outside 0..253402300799, which GATES rejects: GATES relies on the
; Observed clauses to accept iat.
(assert documented-issuance)
(assert clocks-in-range)
(assert ((_ is Present) (select issued "iat")))
(assert (=> ((_ is Present) (select issued "iat")) ((_ is JInteger) (member-value (select issued "iat")))))
(assert (not (gates-iat issued)))
(check-sat)
