; BEXP7 exp negative
(assert (gates-current-example-aliases (store sample "exp" (Present (JInteger (- 1)))) 1000 "gates-oidc"))
(check-sat)
