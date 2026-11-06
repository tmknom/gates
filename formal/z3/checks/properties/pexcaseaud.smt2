; PEXCASEAUD: aUd ASCII case variant
(assert (gates-exact-name-candidate (store sample "aUd" (Present (JString "bad-alias"))) 1000 "gates-oidc"))
(check-sat)
