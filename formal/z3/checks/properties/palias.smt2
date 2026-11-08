; expect: unsat
; PALIAS (GOJ-CLAIM-NAME): adding any alias of iss/aud/exp/iat/nbf, with any
; value, to any Claims Set yields a set that is not accepted.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(declare-const aliasName String)
(declare-const aliasValue JsonValue)
(assert (claim-alias aliasName))
(assert (gates-current (store anyClaims aliasName (Present aliasValue))
                       arbitraryClock arbitraryAudience))
(check-sat)
