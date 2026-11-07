; PA_IAT Known case-fold alias IAT current rejects
(assert (gates-current-example-aliases (store sample "IAT" (Present (JString "independent-value"))) 1000 "gates-oidc"))
(check-sat)
