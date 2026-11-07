; BEXP5 exp at-now
(assert (gates-current-example-aliases (store sample "exp" (Present (JInteger 1000))) 1000 "gates-oidc"))
(check-sat)
