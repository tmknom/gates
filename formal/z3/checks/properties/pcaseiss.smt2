; PCASEISS: iSs ASCII case variant
(assert (gates-current-example-aliases (store sample "iSs" (Present (JString "bad-alias"))) 1000 "gates-oidc"))
(check-sat)
