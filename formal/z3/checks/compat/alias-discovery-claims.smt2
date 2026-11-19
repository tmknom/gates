; expect: unsat
; trace:checks GOJ-CLAIM-NAME-ALIAS GH-OIDC-DISCOVERY-CLAIMS
; None of the names claims_supported lists is an alias, so no Claim GitHub
; supports is rejected as an alias.
(declare-const name String)
(assert (gh-discovery-claims-name name))
(assert (claim-alias name))
(check-sat)
