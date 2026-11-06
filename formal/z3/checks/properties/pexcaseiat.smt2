; PEXCASEIAT: iAt ASCII case variant
(assert (gates-exact-name-candidate (store sample "iAt" (Present (JString "bad-alias"))) 1000 "gates-oidc"))
(check-sat)
