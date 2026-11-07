(declare-const anyClaims Claims)
(declare-const otherClaims Claims)
; PCONGRUENCE: if every inspected name+JsonValue is equal, decisions equal.
(assert (= (core-view anyClaims) (core-view otherClaims)))
(assert (not (= (gates-current-example-aliases anyClaims 1000 "gates-oidc")
                (gates-current-example-aliases otherClaims 1000 "gates-oidc"))))
(check-sat)
