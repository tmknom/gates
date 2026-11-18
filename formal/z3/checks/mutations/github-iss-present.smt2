; expect: unsat
; checks/compat/iss-claim-presence.smt2 loses its counterexample when
; GH-OIDC-ISS is misread as bad-gh-iss-present, which also requires iss: the
; Possibly Rejects row of GOJ-ISS on GH-OIDC-CLAIM-PRESENCE would then fail
; the check, so the reading of presence cannot change silently.
(assert documented-issuance)
(assert (bad-gh-iss-present issued))
(assert (= (select issued "iss") Absent))
(assert (not (gates-iss issued)))
(check-sat)
