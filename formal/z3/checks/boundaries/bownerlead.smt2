; BOWNERLEAD repository_owner_id leading zeros rejected
(assert (gates-current-example-aliases (store sample "repository_owner_id" (Present (JString "01"))) 1000 "gates-oidc"))
(check-sat)
