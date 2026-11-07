; BATTEMPTOVER run_attempt greater than int64 max rejected
(assert (gates-current-example-aliases (store sample "run_attempt" (Present (JString "9223372036854775808"))) 1000 "gates-oidc"))
(check-sat)
