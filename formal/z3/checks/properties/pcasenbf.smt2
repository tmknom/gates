; PCASENBF: nBf ASCII case variant
(assert (gates-current-example-aliases (store sample "nBf" (Present (JString "bad-alias"))) 1000 "gates-oidc"))
(check-sat)
