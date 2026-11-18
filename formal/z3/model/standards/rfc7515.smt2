; RFC 7515 JSON Web Signature: clauses on the decoded JOSE Header. The clauses
; on signatures and keys are formalized with the signature model. The
; conventions of model/standards/ are in rfc7159.smt2.

; trace:unformalized RFC7515-BASE64URL-FORMAT モデルはデコード後の値を扱い、Base64url の符号化を扱わない
; trace:unformalized RFC7515-JSON-WHITESPACE モデルはデコード後の値を扱い、JSON のテキストの空白を扱わない
; trace:unformalized RFC7515-COMPACT-FORMAT モデルはデコード後の値を扱い、JWS Compact Serialization を扱わない
; trace:unformalized RFC7515-JSON-STRICT モデルは妥当な JSON をデコードした後の値を扱い、JSON のテキストを扱わない
; trace:unformalized RFC7515-JSON-TRAILING-INVALID モデルは妥当な JSON をデコードした後の値を扱い、JSON のテキストを扱わない
; trace:unformalized RFC7515-HEADER-NAME-UNIQUE モデルは名前ごとに一つの値を持つデコード後の JOSE Header を扱い、重複した名前を表さない
; trace:unformalized RFC7515-HEADER-NAME-DUPLICATE モデルは名前ごとに一つの値を持つデコード後の JOSE Header を扱い、重複した名前を表さない
; trace:unformalized RFC7515-KEY-HEADER-PROTECT モデルは JWS Protected Header と JWS Unprotected Header を区別しない
; trace:unformalized RFC7515-CRIT-PROTECTED モデルは JWS Protected Header と JWS Unprotected Header を区別しない
; trace:unformalized RFC7515-TYP-APPLICATION 値を処理する主体の定めで、JOSE Header の値の制約を持たない
; trace:unformalized RFC7515-CTY-APPLICATION 値を処理する主体の定めで、JOSE Header の値の制約を持たない
; trace:unformalized RFC7515-TYP-CASE 大文字小文字を区別しないメディアタイプの比較を、SMT の文字列の述語で表さない
; trace:unformalized RFC7515-CTY-CASE 大文字小文字を区別しないメディアタイプの比較を、SMT の文字列の述語で表さない
; trace:unformalized RFC7515-UNICODE-NON-BMP-PRESERVE モデルの文字列はコードポイントの列で、JSON の実装が文字を保持できないことを表さない
; trace:unformalized RFC7515-UNICODE-NON-BMP-REJECT モデルの文字列はコードポイントの列で、JSON の実装が文字を保持できないことを表さない

; The Header Parameter names RFC 7515 defines.
(define-fun std-jws-defined-name ((n String)) Bool
  (or (= n "alg") (= n "jku") (= n "jwk") (= n "kid") (= n "x5u") (= n "x5c")
      (= n "x5t") (= n "x5t#S256") (= n "typ") (= n "cty") (= n "crit")))
; alg and crit are the defined names designated as MUST be understood.
; understood tells that the implementation understands the parameter name;
; acceptedWith and acceptedWithout are the outcomes with the parameter and
; with it removed.
; trace:formalizes RFC7515-HEADER-DEFINED-UNKNOWN-IGNORE
(define-fun std-jws-defined-unknown-ignore ((name String) (understood Bool) (acceptedWith Bool) (acceptedWithout Bool)) Bool
  (=> (and (std-jws-defined-name name)
           (not (= name "alg")) (not (= name "crit"))
           (not understood))
      (= acceptedWith acceptedWithout)))
; listed tells that crit lists the name.
; trace:formalizes RFC7515-HEADER-EXTENSION-UNKNOWN-IGNORE
(define-fun std-jws-extension-unknown-ignore ((name String) (listed Bool) (understood Bool) (acceptedWith Bool) (acceptedWithout Bool)) Bool
  (=> (and (not (std-jws-defined-name name)) (not listed) (not understood))
      (= acceptedWith acceptedWithout)))

(define-fun header-string-in ((h Header) (name String) (allowed (Array String Bool))) Bool
  (and (string-member h name) (select allowed (string-member-value h name))))
; trace:formalizes RFC7515-ALG-REQUIRED
(define-fun std-jws-alg-present ((h Header)) Bool
  ((_ is Present) (get-member h "alg")))
; implemented is the set of algorithms the implementation implements.
; trace:formalizes RFC7515-ALG-UNDERSTAND
(define-fun std-jws-alg-understood ((h Header) (implemented (Array String Bool))) Bool
  (header-string-in h "alg" implemented))
; StringOrURI is not formalized: a value is any string here.
; trace:formalizes RFC7515-ALG-FORMAT
(define-fun std-jws-alg-format ((h Header)) Bool
  (=> ((_ is Present) (get-member h "alg"))
      (and (string-member h "alg") (ascii-text (string-member-value h "alg")))))
; acceptable is the set of algorithms the application accepts.
; trace:formalizes RFC7515-ALG-APPLICATION-ACCEPTABLE
(define-fun std-jws-alg-application-acceptable ((h Header) (acceptable (Array String Bool))) Bool
  (header-string-in h "alg" acceptable))

; elsewhere is the Condition negated: the application determines the key by
; another means or convention.
; trace:formalizes RFC7515-KEY-IDENTIFY
(define-fun std-jws-key-identify ((h Header) (elsewhere Bool)) Bool
  (=> (not elsewhere)
      (or ((_ is Present) (get-member h "kid"))
          ((_ is Present) (get-member h "jku"))
          ((_ is Present) (get-member h "jwk"))
          ((_ is Present) (get-member h "x5u"))
          ((_ is Present) (get-member h "x5c"))
          ((_ is Present) (get-member h "x5t"))
          ((_ is Present) (get-member h "x5t#S256")))))
; trace:formalizes RFC7515-JKU-OPTIONAL
(define-fun std-jws-jku-omitted ((h Header)) Bool
  (= (get-member h "jku") Absent))
; trace:formalizes RFC7515-JWK-OPTIONAL
(define-fun std-jws-jwk-omitted ((h Header)) Bool
  (= (get-member h "jwk") Absent))
; trace:formalizes RFC7515-X5U-OPTIONAL
(define-fun std-jws-x5u-omitted ((h Header)) Bool
  (= (get-member h "x5u") Absent))
; trace:formalizes RFC7515-X5C-OPTIONAL
(define-fun std-jws-x5c-omitted ((h Header)) Bool
  (= (get-member h "x5c") Absent))
; trace:formalizes RFC7515-X5T-OPTIONAL
(define-fun std-jws-x5t-omitted ((h Header)) Bool
  (= (get-member h "x5t") Absent))
; trace:formalizes RFC7515-X5T-S256-OPTIONAL
(define-fun std-jws-x5t-s256-omitted ((h Header)) Bool
  (= (get-member h "x5t#S256") Absent))
; trace:formalizes RFC7515-KID-FORMAT
(define-fun std-jws-kid-format ((h Header)) Bool
  (string-or-absent h "kid"))
; trace:formalizes RFC7515-KID-OPTIONAL
(define-fun std-jws-kid-omitted ((h Header)) Bool
  (= (get-member h "kid") Absent))

; trace:formalizes RFC7515-TYP-OPTIONAL
(define-fun std-jws-typ-omitted ((h Header)) Bool
  (= (get-member h "typ") Absent))
; trace:formalizes RFC7515-CTY-OPTIONAL
(define-fun std-jws-cty-omitted ((h Header)) Bool
  (= (get-member h "cty") Absent))
(define-fun media-type-prefix-omitted ((h Header) (name String)) Bool
  (=> (string-member h name)
      (let ((s (string-member-value h name)))
        (not (and (str.prefixof "application/" s)
                  (not (str.contains (str.substr s 12 (- (str.len s) 12)) "/")))))))
; trace:formalizes RFC7515-TYP-PREFIX-OMIT
(define-fun std-jws-typ-prefix-omitted ((h Header)) Bool
  (media-type-prefix-omitted h "typ"))
; trace:formalizes RFC7515-CTY-PREFIX-OMIT
(define-fun std-jws-cty-prefix-omitted ((h Header)) Bool
  (media-type-prefix-omitted h "cty"))
; uses is the Condition: the recipient uses the media type value. mediaType is
; the media type the recipient takes the value to stand for.
(define-fun media-type-prefix-restored ((h Header) (name String) (uses Bool) (mediaType String)) Bool
  (=> (and uses (string-member h name)
           (not (str.contains (string-member-value h name) "/")))
      (= mediaType (str.++ "application/" (string-member-value h name)))))
; trace:formalizes RFC7515-TYP-PREFIX-RESTORE
(define-fun std-jws-typ-prefix-restore ((h Header) (uses Bool) (mediaType String)) Bool
  (media-type-prefix-restored h "typ" uses mediaType))
; trace:formalizes RFC7515-CTY-PREFIX-RESTORE
(define-fun std-jws-cty-prefix-restore ((h Header) (uses Bool) (mediaType String)) Bool
  (media-type-prefix-restored h "cty" uses mediaType))

(define-fun crit-names ((h Header)) (Seq String)
  (array-strings (member-value (get-member h "crit"))))
(define-fun crit-array ((h Header)) Bool
  (let ((m (get-member h "crit")))
    (and ((_ is Present) m) ((_ is JStringArray) (member-value m)))))
; understood is the set of extension Header Parameter names the recipient
; understands and supports. Both clauses ask that crit be processed, so that
; a JWS listing an extension the recipient does not understand is invalid.
; trace:formalizes RFC7515-CRIT-UNDERSTAND RFC7515-CRIT-UNSUPPORTED-INVALID
(define-fun std-jws-crit-understood ((h Header) (understood (Array String Bool))) Bool
  (=> (crit-array h)
      (forall ((i Int))
        (=> (and (<= 0 i) (< i (seq.len (crit-names h))))
            (select understood (seq.nth (crit-names h) i))))))
; trace:formalizes RFC7515-CRIT-FORMAT
(define-fun std-jws-crit-format ((h Header)) Bool
  (or (= (get-member h "crit") Absent) (crit-array h)))
; RFC 7518 defines no Header Parameter name for use with JWS, so the names
; are those of RFC 7515.
; trace:formalizes RFC7515-CRIT-REGISTERED-NAME
(define-fun std-jws-crit-registered-name ((h Header)) Bool
  (=> (crit-array h)
      (forall ((i Int))
        (=> (and (<= 0 i) (< i (seq.len (crit-names h))))
            (not (std-jws-defined-name (seq.nth (crit-names h) i)))))))
; trace:formalizes RFC7515-CRIT-DUPLICATE
(define-fun std-jws-crit-duplicate ((h Header)) Bool
  (=> (crit-array h)
      (forall ((i Int) (j Int))
        (=> (and (<= 0 i) (< i j) (< j (seq.len (crit-names h))))
            (not (= (seq.nth (crit-names h) i) (seq.nth (crit-names h) j)))))))
; trace:formalizes RFC7515-CRIT-ABSENT-NAME
(define-fun std-jws-crit-absent-name ((h Header)) Bool
  (=> (crit-array h)
      (forall ((i Int))
        (=> (and (<= 0 i) (< i (seq.len (crit-names h))))
            ((_ is Present) (get-member h (seq.nth (crit-names h) i)))))))
; trace:formalizes RFC7515-CRIT-EMPTY
(define-fun std-jws-crit-empty ((h Header)) Bool
  (=> (crit-array h) (> (seq.len (crit-names h)) 0)))
; invalid is the recipient's choice to treat the JWS as invalid.
; trace:formalizes RFC7515-CRIT-VIOLATION-INVALID
(define-fun std-jws-crit-violation-invalid ((h Header) (invalid Bool)) Bool
  (=> invalid
      (not (and (std-jws-crit-format h) (std-jws-crit-registered-name h)
                (std-jws-crit-duplicate h) (std-jws-crit-absent-name h)
                (std-jws-crit-empty h)))))
; trace:formalizes RFC7515-CRIT-OPTIONAL
(define-fun std-jws-crit-omitted ((h Header)) Bool
  (= (get-member h "crit") Absent))

; The rules are those of RFC 7159.
; trace:formalizes RFC7515-STRING-COMPARE
(define-fun std-jws-string-equal ((a String) (b String)) Bool
  (std-json-string-equal a b))
