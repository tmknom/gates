; PEXCASEEXP: eXp ASCII case variant
(assert (gates-exact-name-candidate (store sample "eXp" (Present (JString "bad-alias"))) 1000 "gates-oidc"))
(check-sat)
