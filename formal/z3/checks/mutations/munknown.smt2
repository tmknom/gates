; MUNKNOWN Bad reject extra: normal accepts and bad rejects same input
(assert (and (gates-current-example-aliases (store sample "extra" (Present (JString "hello"))) 1000 "gates-oidc") (not (bad-reject-extra (store sample "extra" (Present (JString "hello"))) 1000 "gates-oidc"))))
(check-sat)
