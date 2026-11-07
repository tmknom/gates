; BNBF300 nbf=now+300
(assert (gates-current-example-aliases (store sample "nbf" (Present (JInteger 1300))) 1000 "gates-oidc"))
(check-sat)
