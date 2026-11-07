(declare-const anyClaims Claims)
(declare-const changeName String)
(declare-const changeValue MaybeClaim)
; PEXACTIMPLIES Current implies exact-key proposal
(assert (and (gates-current-example-aliases anyClaims 1000 "gates-oidc") (not (gates-exact-name-candidate anyClaims 1000 "gates-oidc"))))
(check-sat)
