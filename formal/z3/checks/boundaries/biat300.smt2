; BIAT300 iat=now+300
(assert (gates-current-example-aliases (store sample "iat" (Present (JInteger 1300))) 1000 "gates-oidc"))
(check-sat)
