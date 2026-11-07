; Explicit comparison profiles. The candidate does not change the current design.
; Strict exact-key proposal: ignore aliases (after raw JSON validation), but
; still require exactly spelled keys such as "iss".
(define-fun gates-exact-name-candidate ((c Claims) (now Int) (expected String)) Bool
  (and (gates-common-except-exp c now expected) (gates-exp c now)))

; Mutations: deliberately wrong acceptance predicates. Each check under
; checks/mutations/ shows that a universal property finds a counterexample
; against one of them, so the property is strong enough to detect that bug.
; Rejects one unknown Claim "extra".
(define-fun bad-reject-extra ((c Claims) (now Int) (expected String)) Bool
  (and (gates-current c now expected)
       (= (get-claim c "extra") Absent)))
; Accepts exp == now (<= instead of <).
(define-fun bad-exp-inclusive ((c Claims) (now Int) (expected String)) Bool
  (and (gates-common-except-exp c now expected)
       (integer-in-range c "exp")
       (<= now (integer-value (claim-value (get-claim c "exp"))))
       (aliases-absent c)))
; Accepts any aud that contains the expected Audience, as RFC 7519 allows.
(define-fun bad-aud-contains ((c Claims) (now Int) (expected String)) Bool
  (and (github-issuer c)
       (standard-aud-selected c expected)
       (gates-iat c now)
       (gates-nbf c now)
       (gates-sub c)
       (canonical-id-claim c "run_id")
       (canonical-id-claim c "run_attempt")
       (gates-exp c now)
       (aliases-absent c)))
; Drops the upper limit of exp.
(define-fun bad-exp-unbounded ((c Claims) (now Int) (expected String)) Bool
  (and (gates-common-except-exp c now expected)
       (let ((m (get-claim c "exp")))
         (and ((_ is Present) m)
              ((_ is JInteger) (claim-value m))
              (< now (integer-value (claim-value m)))))
       (aliases-absent c)))
; Rejects only ASCII aliases, missing the ten with U+017F.
(define-fun bad-ascii-aliases-only ((c Claims) (now Int) (expected String)) Bool
  (and (gates-common-except-exp c now expected)
       (gates-exp c now)
       (aliases-absent
         (store (store (store (store (store (store (store (store (store (store c
           "is\u{17f}" Absent) "iS\u{17f}" Absent) "i\u{17f}s" Absent)
           "i\u{17f}S" Absent) "i\u{17f}\u{17f}" Absent) "Is\u{17f}" Absent)
           "IS\u{17f}" Absent) "I\u{17f}s" Absent) "I\u{17f}S" Absent)
           "I\u{17f}\u{17f}" Absent))))
