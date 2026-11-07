; BAUDEMPTY aud empty array rejected
(assert (gates-current-example-aliases (store sample "aud" (Present (JStringArray (as seq.empty (Seq String))))) 1000 "gates-oidc"))
(check-sat)
