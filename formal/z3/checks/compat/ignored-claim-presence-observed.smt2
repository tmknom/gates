; expect: unsat unsat
; trace:checks GOJ-HEAD-REF GOJ-BASE-REF GOJ-ENVIRONMENT GOJ-ENVIRONMENT-NODE-ID GH-OIDC-CLAIM-PRESENCE-OBSERVED
; GH-OIDC-CLAIM-PRESENCE-OBSERVED records empty head_ref and base_ref, and
; environment and environment_node_id only in a job with an Environment.
; GATES reads none of them, so their presence and values do not matter.
; The first check-sat shows GATES consults none of the names; the second
; that a Claim under any of them, of any value or absent, leaves acceptance as
; it is.
(declare-const name String)
(declare-const member Member)
(define-fun described ((n String)) Bool
  (or (= n "head_ref") (= n "base_ref") (= n "environment")
      (= n "environment_node_id")))
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
