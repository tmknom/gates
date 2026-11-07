; PAUDITNULL actor NULL normalized and independently accepted
(assert (and (gates-current-example-aliases (store sample "actor" (Present JNull)) 1000 "gates-oidc") (= (audit-extract-string (store sample "actor" (Present JNull)) "actor") false) (= (audit-attribute-present (store sample "actor" (Present JNull)) "actor") false)))
(check-sat)
