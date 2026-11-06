; Explicit comparison profiles. The candidate does not change the current design.
; Strict exact-key proposal: ignore unknown case variants (after raw JSON
; validation), but still require exactly spelled keys such as "iss".
(define-fun gates-exact-name-candidate ((c Claims) (now Int) (expected String)) Bool
  (and (gates-common-except-exp c now expected) (gates-exp c now)))
; Mutation: reject an explicitly named unknown Claim "extra".
; This is a representative witness for a general reject-unknown bug,
; not a universal definition of all possible unknown names.
(define-fun bad-reject-extra ((c Claims) (now Int) (expected String)) Bool
  (and (gates-current-example-aliases c now expected)
       (= (get-claim c "extra") Absent)))
; Mutation: treat optional audit actor as mandatory string (wrong).
(define-fun bad-reject-audit-wrong-type ((c Claims) (now Int) (expected String)) Bool
  (and (gates-current-example-aliases c now expected)
       (or (= (get-claim c "actor") Absent)
           (string-claim c "actor"))))
; Mutation: exp == now is accepted (wrong <= instead of <).
(define-fun bad-exp-inclusive ((c Claims) (now Int) (expected String)) Bool
  (and (gates-common-except-exp c now expected)
       (integer-in-range c "exp")
       (<= now (integer-value (claim-value (get-claim c "exp"))))
       (alias-examples-absent c)))
