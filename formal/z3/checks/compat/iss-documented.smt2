; expect: unsat
; trace:checks GOJ-ISS GH-OIDC-ISS
; GATES accepts the iss of every issuance the Documented clauses allow, once
; iss is present (GH-OIDC-CLAIM-PRESENCE).
(assert documented-issuance)
(assert ((_ is Present) (select issued "iss")))
(assert (not (gates-iss issued)))
(check-sat)
