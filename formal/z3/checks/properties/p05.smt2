; P05 Multi-audience satisfies RFC recipient membership but not GATES
(assert (and (standard-aud-selected (store sample "aud" (Present (JStringArray (seq.++ (seq.unit "gates-oidc") (seq.unit "other"))))) "gates-oidc") (not (gates-current-example-aliases (store sample "aud" (Present (JStringArray (seq.++ (seq.unit "gates-oidc") (seq.unit "other"))))) 1000 "gates-oidc"))))
(check-sat)
