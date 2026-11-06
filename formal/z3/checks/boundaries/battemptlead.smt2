; BATTEMPTLEAD run_attempt leading zeros rejected
(assert (gates-current-example-aliases (store sample "run_attempt" (Present (JString "01"))) 1000 "gates-oidc"))
(check-sat)
