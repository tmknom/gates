; PROPOSALS -- a candidate that does not change the current design, and
; deliberately wrong predicates of every layer for checks/mutations/. Each
; mutation check shows that a check finds a counterexample against one of
; them, so the check is strong enough to detect that error.

; Strict exact-key candidate: ignore aliases (after raw JSON validation), but
; still require exactly spelled keys such as "iss".
(define-fun gates-exact-name-candidate ((c Claims) (now Int) (expected String)) Bool
  (and (gates-common-except-exp c expected) (gates-exp c) (gates-time c now)))

; GATES: rejects one unknown Claim "extra".
(define-fun bad-reject-extra ((c Claims) (now Int) (expected String)) Bool
  (and (gates-claims-accepted c now expected)
       (= (get-member c "extra") Absent)))
; GATES: accepts exp == now (<= instead of <).
(define-fun bad-exp-inclusive ((c Claims) (now Int) (expected String)) Bool
  (and (gates-common-except-exp c expected)
       (gates-exp c)
       (<= now (claim-integer c "exp"))
       (gates-time-not-ahead c now)
       (gates-aliases-absent c)))
; GATES: accepts any aud that contains the expected Audience, as RFC 7519 allows.
(define-fun bad-aud-contains ((c Claims) (now Int) (expected String)) Bool
  (and (gates-iss c)
       ((_ is Present) (get-member c "aud"))
       (std-aud-identifies c expected)
       (gates-iat c) (gates-nbf c)
       (gates-sub-format c) (gates-sub-length c) (gates-id-match c)
       (gates-repository-id c) (gates-repository-owner-id c)
       (gates-run-id c) (gates-run-attempt c)
       (gates-exp c) (gates-time c now)
       (gates-aliases-absent c)))
; GATES: drops the upper limit of exp.
(define-fun bad-exp-unbounded ((c Claims) (now Int) (expected String)) Bool
  (and (gates-common-except-exp c expected)
       (let ((m (get-member c "exp")))
         (and ((_ is Present) m) ((_ is JInteger) (member-value m))))
       (gates-time c now)
       (gates-aliases-absent c)))
; GATES: rejects only ASCII aliases, missing the ten with U+017F.
(define-fun bad-ascii-aliases-only ((c Claims) (now Int) (expected String)) Bool
  (and (gates-common-except-exp c expected)
       (gates-exp c)
       (gates-time c now)
       (gates-aliases-absent
         (store (store (store (store (store (store (store (store (store (store c
           "is\u{17f}" Absent) "iS\u{17f}" Absent) "i\u{17f}s" Absent)
           "i\u{17f}S" Absent) "i\u{17f}\u{17f}" Absent) "Is\u{17f}" Absent)
           "IS\u{17f}" Absent) "I\u{17f}s" Absent) "I\u{17f}S" Absent)
           "I\u{17f}\u{17f}" Absent))))
; GATES: compares iss ignoring the case of ASCII letters, by also accepting
; the uppercase spelling of the Issuer.
(define-fun bad-iss-case-insensitive ((c Claims)) Bool
  (or (gates-iss c)
      (= (gates-claim c "iss") (Present (JString "HTTPS://TOKEN.ACTIONS.GITHUBUSERCONTENT.COM")))))
; GATES: reads iss under its fullwidth spelling, which NFKC normalizes to
; iss, when iss itself is missing.
(define-fun bad-claim-name-normalized ((c Claims) (now Int) (expected String)) Bool
  (gates-claims-accepted
    (ite (= (get-member c "iss") Absent)
         (store c "iss" (get-member c "\u{ff49}\u{ff53}\u{ff53}"))
         c)
    now expected))
; GATES: lets the typ value decide acceptance.
(define-fun bad-header-typ-required ((h Header)) Bool
  (and (gates-header-accepted h)
       (= (get-member h "typ") (Present (JString "JWT")))))
; GATES: accepts none besides RS256.
(define-fun bad-header-alg-none ((h Header)) Bool
  (and (or (gates-header-alg h) (= (get-member h "alg") (Present (JString "none"))))
       (gates-header-kid h) (gates-header-cty h) (gates-header-crit h)
       (gates-header-b64 h) (gates-header-key-reference h)
       (gates-header-unknown h)))

; STANDARDS: an nbf leeway clause that allows no leeway.
(define-fun bad-std-nbf-leeway ((leeway Real)) Bool
  (= leeway 0.0))
; STANDARDS: an aud clause that asks for aud whether or not the Issuer issues
; for more than one recipient; it contradicts RFC7519-AUD-OPTIONAL.
(define-fun bad-std-aud-required ((c Claims) (multiRecipientIssuer Bool)) Bool
  ((_ is Present) (get-member c "aud")))

; GITHUB: a Documented iss clause that also requires iss to be present.
(define-fun bad-gh-iss-present ((c Claims)) Bool
  (and (gh-iss c) ((_ is Present) (get-member c "iss"))))
; GITHUB: a Documented sub clause that fixes the immutable form for every
; repository, as if GH-OIDC-SUB-LEGACY-RETAINED did not exist.
(define-fun bad-gh-sub-always-immutable ((form SubjectForm)) Bool
  (= form ImmutableForm))
