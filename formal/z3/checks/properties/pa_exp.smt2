; PA_EXP Known case-fold alias EXP current rejects
(assert (gates-current-example-aliases (store sample "EXP" (Present (JString "independent-value"))) 1000 "gates-oidc"))
(check-sat)
