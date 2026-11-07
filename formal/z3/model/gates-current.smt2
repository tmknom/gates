; Adopted GATES acceptance for decoded Claims Set.
; Source of truth: docs/design/jwt.md §§2.2–2.38.4.
(define-fun gates-exp ((c Claims) (now Int)) Bool
  (and (integer-in-range c "exp")
       (< now (integer-value (claim-value (get-claim c "exp"))))))
(define-fun gates-iat ((c Claims) (now Int)) Bool
  (and (integer-in-range c "iat")
       (<= (integer-value (claim-value (get-claim c "iat"))) (+ now 300))))
(define-fun gates-nbf ((c Claims) (now Int)) Bool
  (or (= (get-claim c "nbf") Absent)
      (and (integer-in-range c "nbf")
           (<= (integer-value (claim-value (get-claim c "nbf")))
               (+ now 300)))))
(define-fun gates-aud ((c Claims) (expected String)) Bool
  (and (> (str.len expected) 0)
       (let ((m (get-claim c "aud")))
         (and ((_ is Present) m)
              (let ((v (claim-value m)))
                (or (and ((_ is JString) v) (= (string-value v) expected))
                    (and ((_ is JStringArray) v)
                         (= (array-strings v) (seq.unit expected)))))))))

; Deterministic sub parser over decoded strings. Split at first '/' after
; 'repo:' and first ':' after it. In each NAME@ID: exactly one @, nonempty
; NAME and positive decimal ID. Name is not used for identity/authorization.
; Repo name may contain '/', owner name may contain ':' (per current rules).
; The whole sub is ASCII only and at most 255 characters (jwt.md §2.2(6)).
(define-fun gates-sub ((c Claims)) Bool
  (and (string-claim c "sub")
       (let ((s (string-claim-value c "sub")))
         (and (ascii-text s)
              (<= (str.len s) 255)
              (str.prefixof "repo:" s)
              (let ((slash (str.indexof s "/" 5)))
                (and (> slash 5)
                     (let ((colon (str.indexof s ":" (+ slash 1))))
                       (and (> colon (+ slash 1))
                            (let ((o (str.substr s 5 (- slash 5)))
                                  (r (str.substr s (+ slash 1) (- colon slash 1))))
                              (let ((oa (str.indexof o "@" 0))
                                    (ra (str.indexof r "@" 0)))
                                (and (> oa 0) (> ra 0)
                                     (= (str.indexof o "@" (+ oa 1)) (- 1))
                                     (= (str.indexof r "@" (+ ra 1)) (- 1))
                                     (positive-int64-text (str.substr o (+ oa 1) (- (str.len o) oa 1)))
                                     (positive-int64-text (str.substr r (+ ra 1) (- (str.len r) ra 1)))
                                     (canonical-id-claim c "repository_owner_id")
                                     (canonical-id-claim c "repository_id")
                                     (= (string-claim-value c "repository_owner_id")
                                        (str.substr o (+ oa 1) (- (str.len o) oa 1)))
                                     (= (string-claim-value c "repository_id")
                                        (str.substr r (+ ra 1) (- (str.len r) ra 1))))))))))))))
; The 45 names that claim-alias holds for, listed so that acceptance reads only
; fixed names and needs no quantifier (Z3 answers unknown with one).
; PALIAS shows every claim-alias name is listed, and PCONGRUENCE shows no
; other name affects acceptance, so this list and claim-alias are the same set.
(define-fun aliases-absent ((c Claims)) Bool
  (and
    (= (get-claim c "isS") Absent) (= (get-claim c "is\u{17f}") Absent)
    (= (get-claim c "iSs") Absent) (= (get-claim c "iSS") Absent)
    (= (get-claim c "iS\u{17f}") Absent) (= (get-claim c "i\u{17f}s") Absent)
    (= (get-claim c "i\u{17f}S") Absent) (= (get-claim c "i\u{17f}\u{17f}") Absent)
    (= (get-claim c "Iss") Absent) (= (get-claim c "IsS") Absent)
    (= (get-claim c "Is\u{17f}") Absent) (= (get-claim c "ISs") Absent)
    (= (get-claim c "ISS") Absent) (= (get-claim c "IS\u{17f}") Absent)
    (= (get-claim c "I\u{17f}s") Absent) (= (get-claim c "I\u{17f}S") Absent)
    (= (get-claim c "I\u{17f}\u{17f}") Absent)
    (= (get-claim c "auD") Absent) (= (get-claim c "aUd") Absent)
    (= (get-claim c "aUD") Absent) (= (get-claim c "Aud") Absent)
    (= (get-claim c "AuD") Absent) (= (get-claim c "AUd") Absent)
    (= (get-claim c "AUD") Absent)
    (= (get-claim c "exP") Absent) (= (get-claim c "eXp") Absent)
    (= (get-claim c "eXP") Absent) (= (get-claim c "Exp") Absent)
    (= (get-claim c "ExP") Absent) (= (get-claim c "EXp") Absent)
    (= (get-claim c "EXP") Absent)
    (= (get-claim c "iaT") Absent) (= (get-claim c "iAt") Absent)
    (= (get-claim c "iAT") Absent) (= (get-claim c "Iat") Absent)
    (= (get-claim c "IaT") Absent) (= (get-claim c "IAt") Absent)
    (= (get-claim c "IAT") Absent)
    (= (get-claim c "nbF") Absent) (= (get-claim c "nBf") Absent)
    (= (get-claim c "nBF") Absent) (= (get-claim c "Nbf") Absent)
    (= (get-claim c "NbF") Absent) (= (get-claim c "NBf") Absent)
    (= (get-claim c "NBF") Absent)))
(define-fun gates-common-except-exp ((c Claims) (now Int) (expected String)) Bool
  (and (github-issuer c)
       (gates-aud c expected)
       (gates-iat c now)
       (gates-nbf c now)
       (gates-sub c)
       (canonical-id-claim c "run_id")
       (canonical-id-claim c "run_attempt")))
(define-fun gates-current ((c Claims) (now Int) (expected String)) Bool
  (and (gates-common-except-exp c now expected)
       (gates-exp c now)
       (aliases-absent c)))
