; GATES, JOSE HEADER -- the GOJ acceptance rules on the decoded JOSE Header.
; Source of truth: docs/design/gates-oidc-jwt-signature.md. Each predicate is
; one GOJ decision; gates-header-accepted is their conjunction. The rules on
; Discovery, JWKs and the signature are formalized with the signature model.

; trace:unformalized GOJ-SERIALIZATION-COMPACT モデルはデコード後の値を扱い、JWS Compact Serialization と Base64url を扱わない
; trace:unformalized GOJ-HEADER-JSON モデルは妥当で重複のない JSON をデコードした後の値を扱い、JSON のテキスト、UTF-8、重複した名前を扱わない

(define-fun gates-header-algorithm () String "RS256")
; trace:formalizes GOJ-HEADER-ALG
(define-fun gates-header-alg ((h Header)) Bool
  (= (get-member h "alg") (Present (JString gates-header-algorithm))))
; trace:formalizes GOJ-HEADER-KID
(define-fun gates-header-kid ((h Header)) Bool
  (and (string-member h "kid")
       (> (str.len (string-member-value h "kid")) 0)))
; GATES reads neither the value nor the presence of typ.
; trace:formalizes GOJ-HEADER-TYP
(define-fun gates-header-typ-name ((n String)) Bool
  (= n "typ"))
; trace:formalizes GOJ-HEADER-CTY
(define-fun gates-header-cty ((h Header)) Bool
  (= (get-member h "cty") Absent))
; trace:formalizes GOJ-HEADER-CRIT
(define-fun gates-header-crit ((h Header)) Bool
  (= (get-member h "crit") Absent))
; trace:formalizes GOJ-HEADER-B64
(define-fun gates-header-b64 ((h Header)) Bool
  (= (get-member h "b64") Absent))
; jku, x5u, x5t and x5t#S256 are not read. jwk must read as a JWK and x5c as
; an array of X.509 certificates; the model takes any object as the former and
; any non-empty array of strings as the latter.
; trace:formalizes GOJ-HEADER-KEY-REFERENCE
(define-fun gates-header-key-reference ((h Header)) Bool
  (and (let ((m (get-member h "jwk")))
         (or (= m Absent) (= (member-value m) JObject)))
       (let ((m (get-member h "x5c")))
         (or (= m Absent)
             (and ((_ is JStringArray) (member-value m))
                  (> (seq.len (array-strings (member-value m))) 0))))))
; Unknown Header Parameters are not read, except that nonce must be a string.
; trace:formalizes GOJ-HEADER-UNKNOWN
(define-fun gates-header-unknown ((h Header)) Bool
  (string-or-absent h "nonce"))
; Every Header Parameter name the acceptance predicate reads.
(define-fun header-consulted-name ((n String)) Bool
  (or (= n "alg") (= n "kid") (= n "cty") (= n "crit") (= n "b64")
      (= n "jwk") (= n "x5c") (= n "nonce")))
(define-fun gates-header-accepted ((h Header)) Bool
  (and (gates-header-alg h)
       (gates-header-kid h)
       (gates-header-cty h)
       (gates-header-crit h)
       (gates-header-b64 h)
       (gates-header-key-reference h)
       (gates-header-unknown h)))
