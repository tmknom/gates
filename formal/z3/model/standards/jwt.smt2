; STANDARDS -- clauses of RFC 7519 and RFC 8725 on the decoded Claims Set.
; Each predicate is written from its clause alone and holds no GATES value
; (Issuer, Audience, leeway). A MAY or OPTIONAL clause is the predicate of the
; choice it permits, and a leeway the standard leaves open is a parameter.
; A clause whose actor is the Issuer constrains the issued Claims Set; a
; clause whose actor is the Recipient constrains what it may accept.

; trace:formalizes RFC7519-NUMERICDATE-FORMAT
(define-fun std-numericdate ((v JsonValue)) Bool
  (or ((_ is JInteger) v) ((_ is JFraction) v) ((_ is JExponent) v)))
; Seconds that a NumericDate value stands for. Defined only for std-numericdate.
(define-fun numericdate-seconds ((v JsonValue)) Real
  (ite ((_ is JInteger) v) (to_real (integer-value v))
       (ite ((_ is JFraction) v) (fraction-value v) (exponent-value v))))
(define-fun std-claim-numericdate ((c Claims) (name String)) Bool
  (let ((m (get-claim c name)))
    (or (= m Absent) (std-numericdate (claim-value m)))))

; trace:formalizes RFC7519-EXP-NUMERICDATE
(define-fun std-exp-numericdate ((c Claims)) Bool
  (std-claim-numericdate c "exp"))
; trace:formalizes RFC7519-EXP-OPTIONAL
(define-fun std-exp-omitted ((c Claims)) Bool
  (= (get-claim c "exp") Absent))
; "Usually no more than a few minutes" is not normative, so any leeway from 0 is allowed.
; trace:formalizes RFC7519-EXP-LEEWAY
(define-fun std-exp-leeway ((leeway Real)) Bool
  (>= leeway 0.0))
; The leeway relaxes the clause; leeway 0 is the clause without it.
; trace:formalizes RFC7519-EXP-REJECT
(define-fun std-exp-reject ((c Claims) (now Int) (leeway Real)) Bool
  (let ((m (get-claim c "exp")))
    (=> (and ((_ is Present) m) (std-numericdate (claim-value m)))
        (< (to_real now) (+ (numericdate-seconds (claim-value m)) leeway)))))

; trace:formalizes RFC7519-NBF-NUMERICDATE
(define-fun std-nbf-numericdate ((c Claims)) Bool
  (std-claim-numericdate c "nbf"))
; trace:formalizes RFC7519-NBF-OPTIONAL
(define-fun std-nbf-omitted ((c Claims)) Bool
  (= (get-claim c "nbf") Absent))
; "Usually no more than a few minutes" is not normative, so any leeway from 0 is allowed.
; trace:formalizes RFC7519-NBF-LEEWAY
(define-fun std-nbf-leeway ((leeway Real)) Bool
  (>= leeway 0.0))
; The leeway relaxes the clause; leeway 0 is the clause without it.
; trace:formalizes RFC7519-NBF-REJECT
(define-fun std-nbf-reject ((c Claims) (now Int) (leeway Real)) Bool
  (let ((m (get-claim c "nbf")))
    (=> (and ((_ is Present) m) (std-numericdate (claim-value m)))
        (<= (numericdate-seconds (claim-value m)) (+ (to_real now) leeway)))))

; trace:formalizes RFC7519-IAT-NUMERICDATE
(define-fun std-iat-numericdate ((c Claims)) Bool
  (std-claim-numericdate c "iat"))
; trace:formalizes RFC7519-IAT-OPTIONAL
(define-fun std-iat-omitted ((c Claims)) Bool
  (= (get-claim c "iat") Absent))

; StringOrURI is not formalized: an element is any string here.
; trace:formalizes RFC7519-AUD-FORMAT
(define-fun std-aud-format ((v JsonValue)) Bool
  ((_ is JStringArray) v))
; trace:formalizes RFC7519-AUD-SINGLE-STRING
(define-fun std-aud-single-string ((v JsonValue)) Bool
  ((_ is JString) v))
; trace:formalizes RFC7519-AUD-OPTIONAL
(define-fun std-aud-omitted ((c Claims)) Bool
  (= (get-claim c "aud") Absent))
(define-fun aud-identifies ((v JsonValue) (recipient String)) Bool
  (or (and ((_ is JString) v) (= (string-value v) recipient))
      (and ((_ is JStringArray) v)
           (seq.contains (array-strings v) (seq.unit recipient)))))
; Both clauses ask a recipient processing a JWT with aud to find itself in aud.
; They differ in who is bound, which a model of one recipient does not tell apart.
; trace:formalizes RFC7519-AUD-INTENDED RFC7519-AUD-REJECT
(define-fun std-aud-identifies ((c Claims) (recipient String)) Bool
  (let ((m (get-claim c "aud")))
    (=> ((_ is Present) m) (aud-identifies (claim-value m) recipient))))
; multiRecipientIssuer is the clause's Condition: the Issuer can issue JWTs
; intended for more than one relying party or application.
; Only the presence of aud is formalized. That the value tells an intended
; party from an unintended one depends on which values recipients identify
; themselves with, which a model of one recipient does not represent.
; trace:formalizes RFC8725-AUD-REQUIRED
(define-fun std-aud-required ((c Claims) (multiRecipientIssuer Bool)) Bool
  (=> multiRecipientIssuer ((_ is Present) (get-claim c "aud"))))
; trace:formalizes RFC8725-AUD-VALIDATE
(define-fun std-aud-validate ((c Claims) (recipient String) (multiRecipientIssuer Bool)) Bool
  (=> multiRecipientIssuer
      (let ((m (get-claim c "aud")))
        (and ((_ is Present) m) (aud-identifies (claim-value m) recipient)))))
