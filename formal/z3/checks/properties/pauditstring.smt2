; PAUDITSTRING actor STRING normalized and independently accepted
(assert (and (gates-current-example-aliases (store sample "actor" (Present (JString "octocat"))) 1000 "gates-oidc") (= (audit-extract-string (store sample "actor" (Present (JString "octocat"))) "actor") true) (= (audit-attribute-present (store sample "actor" (Present (JString "octocat"))) "actor") true)))
(check-sat)
