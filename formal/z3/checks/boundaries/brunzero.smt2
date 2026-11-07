; BRUNZERO run_id zero spelling rejected
(assert (gates-current-example-aliases (store sample "run_id" (Present (JString "0"))) 1000 "gates-oidc"))
(check-sat)
