; BSUBNAMESEP owner colon and repo slash remain permitted per current split rules
(assert (gates-current-example-aliases (store sample "sub" (Present (JString "repo:o:x@1/r/name@2:ctx"))) 1000 "gates-oidc"))
(check-sat)
