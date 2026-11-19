; expect: unsat
; trace:checks GOJ-SUB-FORMAT
; The sub of the official example JWT has the legacy form
; (GH-OIDC-SUB-EXAMPLE-LEGACY), which GATES rejects (the Possibly Rejects row
; of GOJ-SUB-FORMAT on GH-OIDC-SUB-LEGACY-RETAINED).
(assert (gates-sub-format gh-example-claims))
(check-sat)
