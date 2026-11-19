; expect: unsat
; trace:checks GOJ-DYNAMIC-CLAIM GOJ-CLAIM-NAME-ALIAS RFC7519-CLAIM-UNKNOWN-IGNORE
; GATES requires the Claims it consults to be processed: the names the
; required and verified rules read, and the aliases, which it understands in
; order to reject them. Any other Claim, of any value, leaves acceptance as it
; is without the Claim, so the clause holds for every name. with-claim and
; without-claim set and remove the Claim only under a name GATES does not
; consult, where the clause applies; under a consulted name both are
; anyClaims, and the clause does not apply. Z3 answers unknown when the store
; is under any name.
(declare-const anyClaims Claims)
(declare-const arbitraryClock Int)
(declare-const arbitraryAudience String)
(declare-const name String)
(declare-const value JsonValue)
(define-fun replaced ((n String)) Bool
  (and (= n name) (not (consulted-name n))))
(define-fun with-claim () Claims
  (lambda ((n String)) (ite (replaced n) (Present value) (select anyClaims n))))
(define-fun without-claim () Claims
  (lambda ((n String)) (ite (replaced n) Absent (select anyClaims n))))
; Only the consulted names count as understood. Taking fewer names as
; understood makes the clause apply to more names, so the check is stronger
; than one that counts the audit and unused Claims as understood too.
(assert (not (std-claim-unknown-ignore
               (consulted-name name)
               (consulted-name name)
               (gates-claims-accepted with-claim arbitraryClock arbitraryAudience)
               (gates-claims-accepted without-claim arbitraryClock arbitraryAudience))))
(check-sat)
