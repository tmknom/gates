; P04 A singleton aud array is accepted
(assert (gates-current-example-aliases (store sample "aud" (Present (JStringArray (seq.unit "gates-oidc")))) 1000 "gates-oidc"))
(check-sat)
