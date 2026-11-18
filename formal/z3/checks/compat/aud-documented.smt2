; expect: unsat
; trace:checks GOJ-AUD GH-OIDC-AUD-CUSTOM
; GATES accepts the aud of every issuance the Documented clauses allow for a
; job that requests the GATES Audience, once aud is present
; (GH-OIDC-CLAIM-PRESENCE). The GATES Audience is not empty.
(assert documented-issuance)
(assert (> (str.len expected) 0))
(assert ((_ is Present) (select issued "aud")))
(assert (not (gates-aud issued expected)))
(check-sat)
