; PEXCASENBF: nBf ASCII case variant
(assert (gates-exact-name-candidate (store sample "nBf" (Present (JString "bad-alias"))) 1000 "gates-oidc"))
(check-sat)
