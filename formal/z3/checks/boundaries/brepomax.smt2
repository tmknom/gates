; BREPOMAX repository_id max positive int64 allowed (if sub matches)
(assert (gates-current-example-aliases (store sample "repository_id" (Present (JString "9223372036854775807"))) 1000 "gates-oidc"))
(check-sat)
