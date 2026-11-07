; BEXPEXP exp 1.001e3 is classified exponent and rejected
(assert (gates-current-example-aliases (store sample "exp" (Present (JExponent 1001.0))) 1000 "gates-oidc"))
(check-sat)
