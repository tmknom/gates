; BNBFNULL nbf null is rejected, not treated missing
(assert (gates-current-example-aliases (store sample "nbf" (Present JNull)) 1000 "gates-oidc"))
(check-sat)
