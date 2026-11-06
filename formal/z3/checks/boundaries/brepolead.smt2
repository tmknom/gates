; BREPOLEAD repository_id leading zeros rejected
(assert (gates-current-example-aliases (store sample "repository_id" (Present (JString "01"))) 1000 "gates-oidc"))
(check-sat)
