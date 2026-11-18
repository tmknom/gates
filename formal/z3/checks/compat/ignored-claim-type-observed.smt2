; expect: unsat unsat
; trace:checks GOJ-REF-PROTECTED GH-OIDC-CLAIM-TYPE-OBSERVED
; GH-OIDC-CLAIM-TYPE-OBSERVED records ref_protected as the string false, not
; a boolean. GATES does not use it, so any type is accepted.
; The first check-sat shows GATES consults none of the names; the second
; that a Claim under any of them, of any value or absent, leaves acceptance as
; it is.
(declare-const name String)
(declare-const member Member)
(define-fun described ((n String)) Bool
  (= n "ref_protected"))
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
