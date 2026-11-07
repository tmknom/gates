; BAUDWRONG aud wrong scalar string rejected
(assert (gates-current-example-aliases (store sample "aud" (Present (JString "other"))) 1000 "gates-oidc"))
(check-sat)
