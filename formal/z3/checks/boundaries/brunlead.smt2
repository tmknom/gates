; BRUNLEAD run_id leading zeros rejected
(assert (gates-current-example-aliases (store sample "run_id" (Present (JString "01"))) 1000 "gates-oidc"))
(check-sat)
