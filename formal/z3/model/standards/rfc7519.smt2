; RFC 7519 JSON Web Token: clauses on the decoded JOSE Header and Claims Set.
; The conventions of model/standards/ are in rfc7159.smt2.

; trace:unformalized RFC7519-SERIALIZATION モデルはデコード後の値を扱い、JWT の直列化を扱わない

; StringOrURI is not formalized: a value is any string here.
; trace:unformalized RFC7519-STRINGORURI-ARBITRARY モデルは StringOrURI を任意の文字列として扱い、URI かどうかを区別しない
; trace:unformalized RFC7519-STRINGORURI-URI URI の構文を SMT の文字列の述語で表さない
; Case-sensitive comparison with no transformation is equality of the strings.
; trace:formalizes RFC7519-STRINGORURI-COMPARE
(define-fun std-stringoruri-equal ((a String) (b String)) Bool
  (= a b))

; trace:formalizes RFC7519-NUMERICDATE-FORMAT
(define-fun std-numericdate ((v JsonValue)) Bool
  (json-number v))
; Seconds that a NumericDate value stands for. Defined only for std-numericdate.
(define-fun numericdate-seconds ((v JsonValue)) Real
  (json-number-seconds v))
(define-fun std-claim-numericdate ((c Claims) (name String)) Bool
  (let ((m (get-member c name)))
    (or (= m Absent) (std-numericdate (member-value m)))))

; trace:unformalized RFC7519-CLAIM-NAME-UNIQUE モデルは名前ごとに一つの値を持つデコード後の Claims Set を扱い、重複した名前を表さない
; trace:unformalized RFC7519-CLAIM-NAME-DUPLICATE モデルは名前ごとに一つの値を持つデコード後の Claims Set を扱い、重複した名前を表さない
; required is the clause's Condition negated: the application requires the
; Claim to be understood and processed in a particular way. understood tells
; that the implementation understands the Claim. acceptedWith and
; acceptedWithout are the outcomes with the Claim and with it removed.
; trace:formalizes RFC7519-CLAIM-UNKNOWN-IGNORE
(define-fun std-claim-unknown-ignore ((required Bool) (understood Bool) (acceptedWith Bool) (acceptedWithout Bool)) Bool
  (=> (and (not required) (not understood)) (= acceptedWith acceptedWithout)))

; trace:formalizes RFC7519-ISS-FORMAT
(define-fun std-iss-format ((c Claims)) Bool
  (string-or-absent c "iss"))
; trace:formalizes RFC7519-ISS-OPTIONAL
(define-fun std-iss-omitted ((c Claims)) Bool
  (= (get-member c "iss") Absent))

; trace:unformalized RFC7519-SUB-UNIQUE 一つの Claims Set を扱うモデルは、Issuer の中で sub が一意であることを表さない
; trace:formalizes RFC7519-SUB-FORMAT
(define-fun std-sub-format ((c Claims)) Bool
  (string-or-absent c "sub"))
; trace:formalizes RFC7519-SUB-OPTIONAL
(define-fun std-sub-omitted ((c Claims)) Bool
  (= (get-member c "sub") Absent))

; StringOrURI is not formalized: an element is any string here.
; trace:formalizes RFC7519-AUD-FORMAT
(define-fun std-aud-format ((v JsonValue)) Bool
  ((_ is JStringArray) v))
; single is the Condition: the JWT has one audience.
; trace:formalizes RFC7519-AUD-SINGLE-STRING
(define-fun std-aud-single-string ((v JsonValue) (single Bool)) Bool
  (and single ((_ is JString) v)))
; trace:formalizes RFC7519-AUD-OPTIONAL
(define-fun std-aud-omitted ((c Claims)) Bool
  (= (get-member c "aud") Absent))
(define-fun aud-identifies ((v JsonValue) (recipient String)) Bool
  (or (and ((_ is JString) v) (std-stringoruri-equal (string-value v) recipient))
      (and ((_ is JStringArray) v)
           (seq.contains (array-strings v) (seq.unit recipient)))))
; Both clauses ask a recipient processing a JWT with aud to find itself in aud.
; They differ in who is bound, which a model of one recipient does not tell apart.
; trace:formalizes RFC7519-AUD-INTENDED RFC7519-AUD-REJECT
(define-fun std-aud-identifies ((c Claims) (recipient String)) Bool
  (let ((m (get-member c "aud")))
    (=> ((_ is Present) m) (aud-identifies (member-value m) recipient))))

; trace:formalizes RFC7519-EXP-NUMERICDATE
(define-fun std-exp-numericdate ((c Claims)) Bool
  (std-claim-numericdate c "exp"))
; trace:formalizes RFC7519-EXP-OPTIONAL
(define-fun std-exp-omitted ((c Claims)) Bool
  (= (get-member c "exp") Absent))
; "Usually no more than a few minutes" is not normative, so any leeway from 0 is allowed.
; trace:formalizes RFC7519-EXP-LEEWAY
(define-fun std-exp-leeway ((leeway Real)) Bool
  (>= leeway 0.0))
; The leeway relaxes the clause; leeway 0 is the clause without it.
; trace:formalizes RFC7519-EXP-REJECT
(define-fun std-exp-reject ((c Claims) (now Int) (leeway Real)) Bool
  (let ((m (get-member c "exp")))
    (=> (and ((_ is Present) m) (std-numericdate (member-value m)))
        (< (to_real now) (+ (numericdate-seconds (member-value m)) leeway)))))

; trace:formalizes RFC7519-NBF-NUMERICDATE
(define-fun std-nbf-numericdate ((c Claims)) Bool
  (std-claim-numericdate c "nbf"))
; trace:formalizes RFC7519-NBF-OPTIONAL
(define-fun std-nbf-omitted ((c Claims)) Bool
  (= (get-member c "nbf") Absent))
; "Usually no more than a few minutes" is not normative, so any leeway from 0 is allowed.
; trace:formalizes RFC7519-NBF-LEEWAY
(define-fun std-nbf-leeway ((leeway Real)) Bool
  (>= leeway 0.0))
; The leeway relaxes the clause; leeway 0 is the clause without it.
; trace:formalizes RFC7519-NBF-REJECT
(define-fun std-nbf-reject ((c Claims) (now Int) (leeway Real)) Bool
  (let ((m (get-member c "nbf")))
    (=> (and ((_ is Present) m) (std-numericdate (member-value m)))
        (<= (numericdate-seconds (member-value m)) (+ (to_real now) leeway)))))

; trace:formalizes RFC7519-IAT-NUMERICDATE
(define-fun std-iat-numericdate ((c Claims)) Bool
  (std-claim-numericdate c "iat"))
; trace:formalizes RFC7519-IAT-OPTIONAL
(define-fun std-iat-omitted ((c Claims)) Bool
  (= (get-member c "iat") Absent))

; trace:unformalized RFC7519-JTI-UNIQUE 一つの Claims Set を扱うモデルは、JWT の間で値が衝突しないことを表さない
; trace:unformalized RFC7519-JTI-ISSUERS-UNIQUE 一つの Claims Set を扱うモデルは、Issuer の間で値が衝突しないことを表さない
; trace:formalizes RFC7519-JTI-FORMAT
(define-fun std-jti-format ((c Claims)) Bool
  (string-or-absent c "jti"))
; trace:formalizes RFC7519-JTI-OPTIONAL
(define-fun std-jti-omitted ((c Claims)) Bool
  (= (get-member c "jti") Absent))

; trace:unformalized RFC7519-TYP-APPLICATION 値を処理する主体の定めで、JOSE Header の値の制約を持たない
; The recommended value is JWT, spelled in uppercase.
; trace:formalizes RFC7519-TYP-VALUE RFC7519-TYP-UPPERCASE
(define-fun std-jwt-typ-value ((h Header)) Bool
  (let ((m (get-member h "typ")))
    (=> ((_ is Present) m) (= (member-value m) (JString "JWT")))))
; trace:formalizes RFC7519-TYP-OPTIONAL
(define-fun std-jwt-typ-omitted ((h Header)) Bool
  (= (get-member h "typ") Absent))
; nested is the Condition: nested signing or encryption is employed.
; trace:formalizes RFC7519-CTY-NOT-NESTED
(define-fun std-jwt-cty-not-nested ((h Header) (nested Bool)) Bool
  (=> (not nested) (= (get-member h "cty") Absent)))
; trace:formalizes RFC7519-CTY-NESTED-PRESENT
(define-fun std-jwt-cty-nested-present ((h Header) (nested Bool)) Bool
  (=> nested ((_ is Present) (get-member h "cty"))))
; The required value is JWT, spelled in uppercase.
; trace:formalizes RFC7519-CTY-NESTED-VALUE RFC7519-CTY-UPPERCASE
(define-fun std-jwt-cty-nested-value ((h Header) (nested Bool)) Bool
  (=> nested (= (get-member h "cty") (Present (JString "JWT")))))

; An Unsecured JWT is a JWS with alg none.
; trace:formalizes RFC7519-UNSECURED-CREATE
(define-fun std-jwt-unsecured ((h Header)) Bool
  (= (get-member h "alg") (Present (JString "none"))))

; trace:unformalized RFC7519-VALIDATE-REJECT §7.2 の手順は直列化、Base64url、JSON の解析と署名の検証で、デコード後の値を扱うモデルの外にある
; acceptable is the set of algorithms the application accepts.
; trace:formalizes RFC7519-ALG-ACCEPTABLE
(define-fun std-jwt-alg-acceptable ((h Header) (acceptable (Array String Bool))) Bool
  (and (string-member h "alg")
       (select acceptable (string-member-value h "alg"))))

; The rules are those of RFC 7159.
; trace:formalizes RFC7519-STRING-COMPARE
(define-fun std-jwt-string-equal ((a String) (b String)) Bool
  (std-json-string-equal a b))
