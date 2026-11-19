; expect: unsat
; checks/compat/sub-format-legacy-retained.smt2 loses its counterexample when
; the sub clauses are misread as bad-gh-sub-always-immutable, which gives every
; repository the immutable form: the Possibly Rejects row of GOJ-SUB-FORMAT on
; GH-OIDC-SUB-LEGACY-RETAINED would then fail the check.
(assert documented-issuance)
(assert (bad-gh-sub-always-immutable form))
(assert (not createdAfter))
(assert (not optedIn))
(assert (not renamed))
(assert (not customized))
(check-sat)
