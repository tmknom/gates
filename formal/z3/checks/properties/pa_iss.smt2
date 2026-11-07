; PA_ISS Known case-fold alias ISS current rejects
(assert (gates-current-example-aliases (store sample "ISS" (Present (JString "independent-value"))) 1000 "gates-oidc"))
(check-sat)
