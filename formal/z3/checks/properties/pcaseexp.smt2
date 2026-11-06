; PCASEEXP: eXp ASCII case variant
(assert (gates-current-example-aliases (store sample "eXp" (Present (JString "bad-alias"))) 1000 "gates-oidc"))
(check-sat)
