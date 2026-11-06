; PAUDSTANDARD RFC recipient membership can hold with 2 aud elements; GATES rejects.
(define-fun c2 () Claims (store sample "aud" (Present (JStringArray (seq.++ (seq.unit "gates-oidc") (seq.unit "other"))))))
(assert (and (standard-aud-selected c2 "gates-oidc")
             (not (gates-current-example-aliases c2 1000 "gates-oidc"))))
(check-sat)
