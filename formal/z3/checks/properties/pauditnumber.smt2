; PAUDITNUMBER actor NUMBER normalized and independently accepted
(assert (and (gates-current-example-aliases (store sample "actor" (Present (JInteger 42))) 1000 "gates-oidc") (= (audit-extract-string (store sample "actor" (Present (JInteger 42))) "actor") false) (= (audit-attribute-present (store sample "actor" (Present (JInteger 42))) "actor") false)))
(check-sat)
