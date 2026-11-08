; RFC-EXP-TYPE/TIME, RFC-NBF-TYPE/TIME, RFC-IAT-TYPE, RFC-ISS/AUD.
; RFC 7519 §2, §4.1.1–4.1.6: https://www.rfc-editor.org/rfc/rfc7519.html
; Chosen leeway: exp=0, nbf=300; RFC MAY permit small leeway but does
; not mandate these numbers. The GATES 300s iat future rule is not RFC MUST.
; These are selected verifier-side conditions, NOT all JWT/OIDC requirements.
(define-fun standard-time-selected ((c Claims) (now Int)) Bool
  (and
    (numeric-date-typed (get-claim c "exp"))
    (< (to_real now) (numeric-date-value (get-claim c "exp")))
    (numeric-date-typed (get-claim c "iat"))
    (or (= (get-claim c "nbf") Absent)
        (and (numeric-date-typed (get-claim c "nbf"))
             (<= (numeric-date-value (get-claim c "nbf"))
                 (+ (to_real now) 300.0))))))
; RFC 7519 §4.1.3: recipient must be among aud when present. Here aud
; is required by the selected GATES/OIDC receiver context.
(define-fun standard-aud-selected ((c Claims) (expected String)) Bool
  (let ((m (get-claim c "aud")))
    (and ((_ is Present) m)
      (let ((v (claim-value m)))
        (or (and ((_ is JString) v) (= (string-value v) expected))
            (and ((_ is JStringArray) v)
                 (seq.contains (array-strings v) (seq.unit expected))))))))
(define-fun standard-selected ((c Claims) (now Int) (expected String)) Bool
  (and (standard-time-selected c now)
       (string-is c "iss" "https://token.actions.githubusercontent.com")
       (standard-aud-selected c expected)
       (string-claim c "sub")))
; OIDC Core §2 sub ASCII <=255, for the ID Token profile.
; GATES adopts the same limit in GOJ-SUB condition 6; PSUBOIDC checks the implication.
(define-fun oidc-sub-conditional ((c Claims)) Bool
  (and (string-claim c "sub")
       (ascii-text (string-claim-value c "sub"))
       (<= (str.len (string-claim-value c "sub")) 255)))
