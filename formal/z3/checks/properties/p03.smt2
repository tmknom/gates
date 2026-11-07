; P03 Current acceptance and selected standard jointly satisfiable
(assert (and (gates-current-example-aliases sample 1000 "gates-oidc") (standard-selected sample 1000 "gates-oidc")))
(check-sat)
