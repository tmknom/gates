; BSUBOLD legacy sub without IDs rejected
(assert (gates-current-example-aliases (store sample "sub" (Present (JString "repo:org/repo:ctx"))) 1000 "gates-oidc"))
(check-sat)
