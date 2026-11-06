; BATTEMPTMAX run_attempt max positive int64 allowed (if sub matches)
(assert (gates-current-example-aliases (store sample "run_attempt" (Present (JString "9223372036854775807"))) 1000 "gates-oidc"))
(check-sat)
