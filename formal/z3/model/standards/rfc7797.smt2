; RFC 7797 JWS Unencoded Payload Option: clauses on b64 in the decoded JOSE
; Header. The conventions of model/standards/ are in rfc7159.smt2.

; trace:formalizes RFC7797-B64-FORMAT
(define-fun std-b64-format ((h Header)) Bool
  (let ((m (get-member h "b64")))
    (or (= m Absent) ((_ is JBoolean) (member-value m)))))
; The value b64 stands for, true when omitted. Defined only for std-b64-format.
(define-fun b64-value ((h Header)) Bool
  (let ((m (get-member h "b64")))
    (ite (= m Absent) true (bool-value (member-value m)))))
; trace:unformalized RFC7797-B64-PROTECTED モデルは JWS Protected Header と JWS Unprotected Header を区別しない
; trace:formalizes RFC7797-B64-OPTIONAL
(define-fun std-b64-omitted ((h Header)) Bool
  (= (get-member h "b64") Absent))
; trace:formalizes RFC7797-CRIT-B64
(define-fun std-b64-crit ((h Header)) Bool
  (=> ((_ is Present) (get-member h "b64"))
      (let ((m (get-member h "crit")))
        (and ((_ is Present) m) (string-array-contains (member-value m) "b64")))))
; trace:unformalized RFC7797-B64-CONSISTENT 一つの JOSE Header を扱うモデルは、同じ文脈の JWS の間で値が変わることを表さない
; trace:formalizes RFC7797-B64-TRUE-OMIT
(define-fun std-b64-true-omit ((h Header)) Bool
  (not (= (get-member h "b64") (Present (JBoolean true)))))
; trace:formalizes RFC7797-JWT-B64-FALSE
(define-fun std-jwt-b64-not-false ((h Header)) Bool
  (not (= (get-member h "b64") (Present (JBoolean false)))))
