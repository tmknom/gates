; expect: unsat unsat
; trace:checks GOJ-TRIGGER-ATTRIBUTES GH-OIDC-CLAIM-PRESENCE GH-OIDC-CLAIM-TYPE
; GitHub fixes neither when the audit Claims are present nor their types.
; GATES forwards only string values as workflow trigger attributes, and the
; acceptance reads none of them.
; The first check-sat shows GATES consults none of the names; the second
; that a Claim under any of them, of any value or absent, leaves acceptance as
; it is.
(declare-const name String)
(declare-const member Member)
(define-fun described ((n String)) Bool
  (gates-audit-name n))
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
