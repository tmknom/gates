; PA_NBF Known case-fold alias NBF current rejects
(assert (gates-current-example-aliases (store sample "NBF" (Present (JString "independent-value"))) 1000 "gates-oidc"))
(check-sat)
