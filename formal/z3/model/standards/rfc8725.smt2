; RFC 8725 JWT Best Current Practices: clauses on the decoded JOSE Header and
; Claims Set. The clauses on keys and cryptographic operations are formalized
; with the signature model. The conventions of model/standards/ are in
; rfc7159.smt2.

; supported is the set of algorithms the caller specifies. SPECIFY asks that
; the set can be given and RESTRICT that no other algorithm is used; for one
; JWT both mean that its algorithm is in the given set.
; trace:formalizes RFC8725-ALG-SET-SPECIFY RFC8725-ALG-SET-RESTRICT
(define-fun std-alg-set-restrict ((h Header) (supported (Array String Bool))) Bool
  (header-string-in h "alg" supported))
; current is the set of cryptographically current algorithms that meet the
; security requirements of the application.
; trace:formalizes RFC8725-ALG-CURRENT
(define-fun std-alg-current ((h Header) (current (Array String Bool))) Bool
  (header-string-in h "alg" current))
; requested is the Condition negated: the caller explicitly requests none.
; trace:formalizes RFC8725-ALG-NONE-GENERATE
(define-fun std-alg-none-generate ((h Header) (requested Bool)) Bool
  (=> (not requested) (not (= (get-member h "alg") (Present (JString "none"))))))
; trace:formalizes RFC8725-ALG-NONE-CONSUME
(define-fun std-alg-none-consume ((h Header) (requested Bool)) Bool
  (=> (not requested) (not (= (get-member h "alg") (Present (JString "none"))))))

; trace:unformalized RFC8725-UTF8-IMPLEMENTATION モデルはデコード後の文字列を扱い、UTF-8 のバイト列を扱わない
; trace:unformalized RFC8725-UTF8-APPLICATION モデルはデコード後の文字列を扱い、UTF-8 のバイト列を扱わない

; validSubject is the set of subjects valid at the application, for the
; issuer of the JWT. The clause applies when the JWT contains sub.
; trace:formalizes RFC8725-SUB-VALIDATE
(define-fun std-sub-validate ((c Claims) (validSubject (Array String Bool))) Bool
  (=> ((_ is Present) (get-member c "sub"))
      (and (string-member c "sub")
           (select validSubject (string-member-value c "sub")))))

; multiRecipientIssuer is the clause's Condition: the Issuer can issue JWTs
; intended for more than one relying party or application.
; Only the presence of aud is formalized. That the value tells an intended
; party from an unintended one depends on which values recipients identify
; themselves with, which a model of one recipient does not represent.
; trace:formalizes RFC8725-AUD-REQUIRED
(define-fun std-aud-required ((c Claims) (multiRecipientIssuer Bool)) Bool
  (=> multiRecipientIssuer ((_ is Present) (get-member c "aud"))))
; trace:formalizes RFC8725-AUD-VALIDATE
(define-fun std-aud-validate ((c Claims) (recipient String) (multiRecipientIssuer Bool)) Bool
  (=> multiRecipientIssuer
      (let ((m (get-member c "aud")))
        (and ((_ is Present) m) (aud-identifies (member-value m) recipient)))))

; The model does not fetch. A recipient follows the URL in jku or x5u only if
; the URL can affect its outcome; followed tells that it does, and allowed
; that the URL is on a list of allowed locations, the protection the clause
; gives as an example.
; trace:formalizes RFC8725-JKU-X5U-SSRF
(define-fun std-jku-x5u-ssrf ((followed Bool) (allowed Bool)) Bool
  (=> followed allowed))

; confusable is the Condition: the JWT could be confused with another kind.
; ofOtherKind tells that the JWT is of the other kind, and accepted that the
; application accepts it as its own kind.
; trace:formalizes RFC8725-KIND-CONFUSION-MITIGATE
(define-fun std-kind-confusion-mitigate ((confusable Bool) (ofOtherKind Bool) (accepted Bool)) Bool
  (=> (and confusable ofOtherKind) (not accepted)))
; sameIssuer is the Condition: one issuer can issue more than one kind.
; acceptedAsOne and acceptedAsOther are the validation rules of two kinds on
; the same JWT.
; trace:formalizes RFC8725-KIND-EXCLUSIVE
(define-fun std-kind-exclusive ((sameIssuer Bool) (acceptedAsOne Bool) (acceptedAsOther Bool)) Bool
  (=> sameIssuer (not (and acceptedAsOne acceptedAsOther))))

; trace:formalizes RFC8725-TYP-PREFIX-OMIT
(define-fun std-typ-prefix-omitted ((h Header)) Bool
  (media-type-prefix-omitted h "typ"))
; explicit is the Condition: the JWT uses explicit typing.
; trace:formalizes RFC8725-TYP-MEDIA-TYPE
(define-fun std-typ-media-type ((h Header) (explicit Bool)) Bool
  (=> explicit
      (and (string-member h "typ")
           (str.suffixof "+jwt" (string-member-value h "typ")))))
; trace:unformalized RFC8725-TYP-NESTED モデルは一つの JOSE Header を扱い、Nested JWT の内側の JWT を表さない
; trace:unformalized RFC8725-TYP-NEW-USE JWT の新しい用途を設計するときの定めで、一つの JWT の受理の判定ではない
