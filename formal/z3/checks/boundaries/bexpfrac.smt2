; BEXPFRAC exp 1001.0 with same mathematical value is rejected
(assert (gates-current-example-aliases (store sample "exp" (Present (JFraction 1001.0))) 1000 "gates-oidc"))
(check-sat)
