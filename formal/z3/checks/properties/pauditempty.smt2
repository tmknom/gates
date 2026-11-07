; PAUDITEMPTY actor EMPTY normalized and independently accepted
(assert (and (gates-current-example-aliases (store sample "actor" (Present (JString ""))) 1000 "gates-oidc") (= (audit-extract-string (store sample "actor" (Present (JString ""))) "actor") true) (= (audit-attribute-present (store sample "actor" (Present (JString ""))) "actor") false)))
(check-sat)
