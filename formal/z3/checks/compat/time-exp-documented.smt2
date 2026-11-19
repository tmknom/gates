; expect: unsat
; trace:checks GOJ-TIME-EXP GH-OIDC-REGISTERED-CLAIMS
; GATES accepts, at now, every issuance the Documented clauses allow for use
; at now, before its exp. An integer exp is the one GOJ-EXP admits. The exp
; decision has no leeway, so a GitHub clock behind the Lambda clock is not
; absorbed (GOJ-TIME-EXP narrows RFC7519-EXP-LEEWAY).
(assert documented-issuance)
(assert (= usedAt now))
(assert (gates-exp issued))
(assert (not (gates-time-exp issued now)))
(check-sat)
