; expect: unsat unsat
; trace:checks GOJ-ACTOR GOJ-ACTOR-ID GH-OIDC-ACTOR
; GH-OIDC-ACTOR gives the meaning of actor and actor_id, which GATES only
; records.
; The first check-sat shows GATES consults none of the names; the second
; that a Claim under any of them, of any value or absent, leaves acceptance as
; it is.
(declare-const name String)
(declare-const member Member)
(define-fun described ((n String)) Bool
  (or (= n "actor") (= n "actor_id")))
(push 1)
(assert (described name))
(assert (consulted-name name))
(check-sat)
(pop 1)
(push 1)
(assert (described name))
(assert (claim-changes-acceptance name member))
(check-sat)
(pop 1)
