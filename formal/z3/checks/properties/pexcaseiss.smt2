; PEXCASEISS: iSs ASCII case variant
(assert (gates-exact-name-candidate (store sample "iSs" (Present (JString "bad-alias"))) 1000 "gates-oidc"))
(check-sat)
