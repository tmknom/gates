; RFC-ISS/SUB-TYPE. RFC 7519 §4.1.1–4.1.2: https://www.rfc-editor.org/rfc/rfc7519.html
; Selected verifier-side conditions on iss and sub, NOT all JWT/OIDC requirements.
; The time and audience clauses are in model/standards/.
(define-fun standard-selected ((c Claims)) Bool
  (and (string-is c "iss" "https://token.actions.githubusercontent.com")
       (string-claim c "sub")))
; OIDC Core §2 sub ASCII <=255, for the ID Token profile.
; GATES adopts the same limit in GOJ-SUB condition 6; PSUBOIDC checks the implication.
(define-fun oidc-sub-conditional ((c Claims)) Bool
  (and (string-claim c "sub")
       (ascii-text (string-claim-value c "sub"))
       (<= (str.len (string-claim-value c "sub")) 255)))
