; PALIASBREAK Adding ISS can change acceptance
(assert (and (gates-current-example-aliases sample 1000 "gates-oidc") (not (gates-current-example-aliases (store sample "ISS" (Present (JString "new"))) 1000 "gates-oidc"))))
(check-sat)
