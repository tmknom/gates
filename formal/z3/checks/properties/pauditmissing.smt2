; PAUDITMISSING actor MISSING normalized and independently accepted
(assert (and (gates-current-example-aliases (store sample "actor" Absent) 1000 "gates-oidc") (= (audit-extract-string (store sample "actor" Absent) "actor") false) (= (audit-attribute-present (store sample "actor" Absent) "actor") false)))
(check-sat)
