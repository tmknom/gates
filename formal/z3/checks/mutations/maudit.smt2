; MAUDIT Bad reject audit wrong type: normal accepts and bad rejects
(assert (and (gates-current-example-aliases (store sample "actor" (Present (JInteger 123))) 1000 "gates-oidc") (not (bad-reject-audit-wrong-type (store sample "actor" (Present (JInteger 123))) 1000 "gates-oidc"))))
(check-sat)
