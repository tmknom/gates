; BSUBEMPTYCTX empty sub context permitted
(assert (gates-current-example-aliases (store sample "sub" (Present (JString "repo:org@1/repo@2:"))) 1000 "gates-oidc"))
(check-sat)
