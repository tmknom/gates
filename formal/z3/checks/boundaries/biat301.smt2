; BIAT301 iat=now+301
(assert (gates-current-example-aliases (store sample "iat" (Present (JInteger 1301))) 1000 "gates-oidc"))
(check-sat)
