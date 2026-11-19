; OpenID Connect Core 1.0: clauses on the decoded ID Token Claims Set and JOSE
; Header, and on string comparison. idToken is the Condition that the JWT is an
; ID Token; the flow Conditions are Bool parameters as well. The clauses on
; signing and keys are formalized with the signature model. The conventions of
; model/standards/ are in rfc7159.smt2.

(define-fun present-member ((o JsonObject) (name String)) Bool
  ((_ is Present) (get-member o name)))

; trace:formalizes OIDCC-ISS-REQUIRED
(define-fun std-oidc-iss-required ((c Claims) (idToken Bool)) Bool
  (=> idToken (present-member c "iss")))
; Only the https scheme and the absence of query and fragment are formalized;
; the URL syntax is not.
; trace:formalizes OIDCC-ISS-FORMAT
(define-fun std-oidc-iss-format ((c Claims) (idToken Bool)) Bool
  (=> (and idToken (present-member c "iss"))
      (and (string-member c "iss")
           (str.prefixof "https://" (string-member-value c "iss"))
           (not (str.contains (string-member-value c "iss") "?"))
           (not (str.contains (string-member-value c "iss") "#")))))
; trace:formalizes OIDCC-SUB-REQUIRED
(define-fun std-oidc-sub-required ((c Claims) (idToken Bool)) Bool
  (=> idToken (present-member c "sub")))
; trace:unformalized OIDCC-SUB-IDENTIFIER 一つの Claims Set を扱うモデルは、Issuer の中で sub が一意で再割当てされないことを表さない
; trace:formalizes OIDCC-SUB-LENGTH
(define-fun std-oidc-sub-length ((c Claims) (idToken Bool)) Bool
  (=> (and idToken (present-member c "sub"))
      (and (string-member c "sub")
           (ascii-text (string-member-value c "sub"))
           (<= (str.len (string-member-value c "sub")) 255))))
; trace:formalizes OIDCC-SUB-FORMAT
(define-fun std-oidc-sub-format ((c Claims) (idToken Bool)) Bool
  (=> idToken (string-or-absent c "sub")))
; trace:formalizes OIDCC-AUD-REQUIRED
(define-fun std-oidc-aud-required ((c Claims) (idToken Bool)) Bool
  (=> idToken (present-member c "aud")))
(define-fun oidc-aud-contains ((c Claims) (value String)) Bool
  (let ((m (get-member c "aud")))
    (and ((_ is Present) m)
         (or (= (member-value m) (JString value))
             (string-array-contains (member-value m) value)))))
; trace:formalizes OIDCC-AUD-CLIENT-ID
(define-fun std-oidc-aud-client-id ((c Claims) (idToken Bool) (clientId String)) Bool
  (=> idToken (oidc-aud-contains c clientId)))
; trace:formalizes OIDCC-AUD-OTHER
(define-fun std-oidc-aud-other ((c Claims) (idToken Bool) (clientId String) (other String)) Bool
  (and idToken (not (= other clientId)) (oidc-aud-contains c other)))
; trace:formalizes OIDCC-AUD-FORMAT
(define-fun std-oidc-aud-format ((c Claims) (idToken Bool)) Bool
  (=> (and idToken (present-member c "aud"))
      ((_ is JStringArray) (member-value (get-member c "aud")))))
; single is the Condition: the ID Token has one audience.
; trace:formalizes OIDCC-AUD-SINGLE-STRING
(define-fun std-oidc-aud-single-string ((c Claims) (idToken Bool) (single Bool)) Bool
  (and idToken single (string-member c "aud")))
; trace:formalizes OIDCC-EXP-REQUIRED
(define-fun std-oidc-exp-required ((c Claims) (idToken Bool)) Bool
  (=> idToken (present-member c "exp")))
; authenticating is the Condition that the Relying Party performs
; authentication with the OpenID Provider.
; trace:formalizes OIDCC-EXP-REJECT
(define-fun std-oidc-exp-reject ((c Claims) (idToken Bool) (authenticating Bool) (now Int) (leeway Real)) Bool
  (let ((m (get-member c "exp")))
    (=> (and idToken authenticating ((_ is Present) m) (json-number (member-value m)))
        (< (to_real now) (+ (json-number-seconds (member-value m)) leeway)))))
; trace:formalizes OIDCC-EXP-LEEWAY
(define-fun std-oidc-exp-leeway ((leeway Real)) Bool
  (>= leeway 0.0))
(define-fun oidc-number-or-absent ((c Claims) (name String)) Bool
  (let ((m (get-member c name)))
    (or (= m Absent) (json-number (member-value m)))))
; trace:formalizes OIDCC-EXP-FORMAT
(define-fun std-oidc-exp-format ((c Claims) (idToken Bool)) Bool
  (=> idToken (oidc-number-or-absent c "exp")))
; trace:formalizes OIDCC-IAT-REQUIRED
(define-fun std-oidc-iat-required ((c Claims) (idToken Bool)) Bool
  (=> idToken (present-member c "iat")))
; trace:formalizes OIDCC-IAT-FORMAT
(define-fun std-oidc-iat-format ((c Claims) (idToken Bool)) Bool
  (=> idToken (oidc-number-or-absent c "iat")))
; requested is the Condition: max_age is requested or auth_time is requested
; as an Essential Claim.
; trace:formalizes OIDCC-AUTH-TIME-REQUIRED
(define-fun std-oidc-auth-time-required ((c Claims) (idToken Bool) (requested Bool)) Bool
  (=> (and idToken requested) (present-member c "auth_time")))
; trace:formalizes OIDCC-AUTH-TIME-OPTIONAL
(define-fun std-oidc-auth-time-omitted ((c Claims) (idToken Bool) (requested Bool)) Bool
  (and idToken (not requested) (= (get-member c "auth_time") Absent)))

; sentNonce is the nonce parameter sent in the Authentication Request.
; trace:formalizes OIDCC-NONCE-VERIFY
(define-fun std-oidc-nonce-verify ((c Claims) (idToken Bool) (sentNonce String)) Bool
  (=> (and idToken (present-member c "nonce"))
      (= (get-member c "nonce") (Present (JString sentNonce)))))
; sent is the Condition that the Authentication Request has a nonce.
; trace:formalizes OIDCC-NONCE-INCLUDE
(define-fun std-oidc-nonce-include ((c Claims) (idToken Bool) (sent Bool) (sentNonce String)) Bool
  (=> (and idToken sent) (= (get-member c "nonce") (Present (JString sentNonce)))))
; trace:unformalized OIDCC-NONCE-NO-PROCESSING Authorization Server の処理の定めで、ID Token の値の制約を持たない
; trace:formalizes OIDCC-NONCE-FORMAT
(define-fun std-oidc-nonce-format ((c Claims) (idToken Bool)) Bool
  (=> idToken (string-or-absent c "nonce")))
; trace:formalizes OIDCC-ACR-OPTIONAL
(define-fun std-oidc-acr-omitted ((c Claims) (idToken Bool)) Bool
  (and idToken (= (get-member c "acr") Absent)))
; trace:unformalized OIDCC-ACR-LEVEL-0 資源の金銭的価値と認可の判断は、ID Token の値の制約ではない
; trace:unformalized OIDCC-ACR-VALUE-FORM 絶対 URI の構文と RFC 6711 の登録を、SMT の文字列の述語で表さない
; trace:unformalized OIDCC-ACR-REGISTERED-MEANING 登録された名前の意味は、ID Token の値の制約ではない
; trace:formalizes OIDCC-AMR-OPTIONAL
(define-fun std-oidc-amr-omitted ((c Claims) (idToken Bool)) Bool
  (and idToken (= (get-member c "amr") Absent)))
; trace:unformalized OIDCC-AMR-REGISTERED IANA の登録を、SMT の文字列の述語で表さない
; trace:formalizes OIDCC-AZP-OPTIONAL
(define-fun std-oidc-azp-omitted ((c Claims) (idToken Bool)) Bool
  (and idToken (= (get-member c "azp") Absent)))
; trace:formalizes OIDCC-AZP-CLIENT-ID
(define-fun std-oidc-azp-client-id ((c Claims) (idToken Bool) (authorizedParty String)) Bool
  (=> (and idToken (present-member c "azp"))
      (= (get-member c "azp") (Present (JString authorizedParty)))))
; trace:unformalized OIDCC-CLAIM-OTHER ほかの Claim を含めてよいという許可で、値の制約を持たない
; understood tells that the Relying Party understands the Claim; acceptedWith
; and acceptedWithout are the outcomes with the Claim and with it removed.
; trace:formalizes OIDCC-CLAIM-UNKNOWN-IGNORE
(define-fun std-oidc-claim-unknown-ignore ((idToken Bool) (understood Bool) (acceptedWith Bool) (acceptedWithout Bool)) Bool
  (=> (and idToken (not understood)) (= acceptedWith acceptedWithout)))
; allowed is the Condition negated: the Response Type returns no ID Token from
; the Authorization Endpoint and the Client requested none at Registration.
; trace:formalizes OIDCC-ALG-NONE
(define-fun std-oidc-alg-none ((h Header) (idToken Bool) (allowed Bool)) Bool
  (=> (and idToken (not allowed))
      (not (= (get-member h "alg") (Present (JString "none"))))))
; trace:formalizes OIDCC-KEY-HEADER-AVOID
(define-fun std-oidc-key-header-avoid ((h Header) (idToken Bool)) Bool
  (=> idToken
      (and (= (get-member h "x5u") Absent) (= (get-member h "x5c") Absent)
           (= (get-member h "jku") Absent) (= (get-member h "jwk") Absent))))

; codeFlow is the Condition that the ID Token is in the Token Response of the
; Authorization Code Flow.
; trace:unformalized OIDCC-CODE-VALIDATE 手順の全体に従う定めで、個々の手順は別の条項が定める
; trace:formalizes OIDCC-CODE-ISS-MATCH
(define-fun std-oidc-code-iss-match ((c Claims) (codeFlow Bool) (issuerIdentifier String)) Bool
  (=> codeFlow (= (get-member c "iss") (Present (JString issuerIdentifier)))))
; trace:formalizes OIDCC-CODE-AUD-CLIENT
(define-fun std-oidc-code-aud-client ((c Claims) (codeFlow Bool) (clientId String)) Bool
  (=> codeFlow (oidc-aud-contains c clientId)))
; trace:formalizes OIDCC-CODE-AUD-MULTIPLE
(define-fun std-oidc-code-aud-multiple ((c Claims) (codeFlow Bool)) Bool
  (let ((m (get-member c "aud")))
    (and codeFlow ((_ is Present) m) ((_ is JStringArray) (member-value m))
         (> (seq.len (array-strings (member-value m))) 1))))
; trusted is the set of audiences the Client trusts, its own client_id included.
; trace:formalizes OIDCC-CODE-AUD-UNTRUSTED
(define-fun std-oidc-code-aud-untrusted ((c Claims) (codeFlow Bool) (trusted (Array String Bool))) Bool
  (let ((m (get-member c "aud")))
    (=> (and codeFlow ((_ is Present) m))
        (or (and ((_ is JString) (member-value m)) (select trusted (string-value (member-value m))))
            (and ((_ is JStringArray) (member-value m))
                 (forall ((i Int))
                   (=> (and (<= 0 i) (< i (seq.len (array-strings (member-value m)))))
                       (select trusted (seq.nth (array-strings (member-value m)) i)))))))))
; trace:unformalized OIDCC-CODE-AZP-EXTENSION 拡張が定める検証は、この仕様の外にある
; trace:formalizes OIDCC-CODE-AZP-CLIENT-ID
(define-fun std-oidc-code-azp-client-id ((c Claims) (codeFlow Bool) (clientId String)) Bool
  (=> (and codeFlow (present-member c "azp"))
      (= (get-member c "azp") (Present (JString clientId)))))
; registered is the algorithm the Client sent in id_token_signed_response_alg,
; or RS256 when it sent none.
; trace:formalizes OIDCC-CODE-ALG-DEFAULT
(define-fun std-oidc-code-alg-default ((h Header) (codeFlow Bool) (registered String)) Bool
  (=> codeFlow (= (get-member h "alg") (Present (JString registered)))))
; OIDCC-EXP-LEEWAY allows a leeway in this decision as well.
; trace:formalizes OIDCC-CODE-EXP
(define-fun std-oidc-code-exp ((c Claims) (codeFlow Bool) (now Int) (leeway Real)) Bool
  (let ((m (get-member c "exp")))
    (=> (and codeFlow ((_ is Present) m) (json-number (member-value m)))
        (< (to_real now) (+ (json-number-seconds (member-value m)) leeway)))))
; trace:formalizes OIDCC-CODE-NONCE
(define-fun std-oidc-code-nonce ((c Claims) (codeFlow Bool) (sent Bool) (sentNonce String)) Bool
  (=> (and codeFlow sent) (= (get-member c "nonce") (Present (JString sentNonce)))))
; trace:unformalized OIDCC-CODE-NONCE-REPLAY 一つの Claims Set を扱うモデルは、ID Token の間のリプレイを表さない
; trace:unformalized OIDCC-CODE-ACR 求めた acr に照らした値の適否の判断は、モデルの外にある
; trace:unformalized OIDCC-CODE-AUTH-TIME 求めた auth_time に照らした値の適否の判断は、モデルの外にある
; trace:unformalized OIDCC-IMPLICIT-VALIDATE 手順の全体に従う定めで、個々の手順は別の条項が定める
; implicitFlow is the Condition that the ID Token is from the Implicit Flow.
; trace:formalizes OIDCC-IMPLICIT-NONCE
(define-fun std-oidc-implicit-nonce ((c Claims) (implicitFlow Bool) (sentNonce String)) Bool
  (=> implicitFlow (= (get-member c "nonce") (Present (JString sentNonce)))))
; trace:unformalized OIDCC-IMPLICIT-NONCE-REPLAY 一つの Claims Set を扱うモデルは、ID Token の間のリプレイを表さない
; trace:unformalized OIDCC-HYBRID-AUTHORIZATION-ENDPOINT 手順の全体に従う定めで、個々の手順は別の条項が定める
; trace:unformalized OIDCC-HYBRID-TOKEN-ENDPOINT 手順の全体に従う定めで、個々の手順は別の条項が定める

; trace:unformalized OIDCC-STRING-UNESCAPE モデルの文字列はエスケープを除いた後のコードポイントの列で、エスケープの除去を表さない
; Equality of code point sequences, with no normalization of either string.
; trace:formalizes OIDCC-STRING-CODE-POINT OIDCC-STRING-NO-NORMALIZATION
(define-fun std-oidc-string-equal ((a String) (b String)) Bool
  (= a b))
; trace:unformalized OIDCC-SPACE-DELIMITER モデルは空白区切りの文字列のリストを扱わない
