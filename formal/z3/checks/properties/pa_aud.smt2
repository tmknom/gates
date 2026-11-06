; PA_AUD Known case-fold alias AUD current rejects
(assert (gates-current-example-aliases (store sample "AUD" (Present (JString "independent-value"))) 1000 "gates-oidc"))
(check-sat)
