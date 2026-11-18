# OpenID Connect Core 1.0

この文書は、OpenID Connect Core から、ID Token と署名に関係する規定を、ID の付いた条項に分けて整理したものです。標準仕様の代替ではなく、規定の適用対象と条件はリンク先の原文を優先します。

- **対象**：ID Token（§2）、Authorization Code Flow、Implicit Flow、Hybrid Flow の ID Token の検証（§3.1.3.7、§3.2.2.11、§3.3.2.12、§3.3.3.7）、署名（§10.1、§10.1.1）、文字列の操作（§14）
- **対象外**：OAuth 2.0 の認可フロー全体、JWE による暗号化（§2 と §10 の署名してから暗号化する規定、§3.1.3.7 の手順 1 を含む）、Access Token の検証と `at_hash`（§3.1.3.6、§3.1.3.8）、認証要求と Token Endpoint の処理、実装の考慮事項と必須の実装機能（§15）、そのほかのセキュリティの考慮事項

各条項の Level は、原文が使う [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119.html) の語か、RFC 2119 の語を使わない定義を表す Definition です。
OpenID Connect Core は、RFC 2119 の語を大文字に限る旨を書いていません。条項にした節の小文字の「may」は可能性を表す言い回し（「which may be context specific」など）で、規範ではありません。

OpenID Connect Core §1.2 は、Relying Party（RP）を、OpenID Provider に End-User の認証と Claim を求める OAuth 2.0 の Client と、OpenID Provider（OP）を、End-User を認証して Relying Party に Claim を提供できる OAuth 2.0 の Authorization Server と定義しています。原文は同じ主体を Client、RP、Relying Party、あるいは OP、Authorization Server、OpenID Provider と呼び分けるため、Actor を Relying Party と OpenID Provider に統一します。

OpenID Connect Core の規定の多くは、ID Token か特定の認証フローに限られます。それらの条項は、Condition に「ID Token」や認証フローの名前を書きます。ID Token でない JWT には、それらの条項は適用されず、JWT 一般の規則（[RFC 7519 JSON Web Token](rfc7519.md)、[RFC 8725 JWT Best Current Practices](rfc8725.md)、[RFC 7515 JSON Web Signature](rfc7515.md) など）が適用されます。たとえば、`aud` に Relying Party の `client_id` を含める要件（OIDCC-AUD-CLIENT-ID）は ID Token の規定です。`aud` がある JWT で受信者が自身を識別できなければ拒否すること（[RFC 7519 JSON Web Token](rfc7519.md) の RFC7519-AUD-REJECT）、鍵が Issuer に属することを検証すること（[RFC 8725 JWT Best Current Practices](rfc8725.md) の RFC8725-ISS-KEY-VALIDATE）は、JWT 一般の規定です。

## 1. ID Token Claims

OpenID Connect Core §2 は、ID Token を、End-User の認証についての Claim を含む JWT と定め、すべての OAuth 2.0 のフローで ID Token に使う Claim を挙げています。OpenID Connect Core は `nbf` を定めていません。この節の条項の Actor は、ID Token を作る OpenID Provider か、ID Token を受け取る Relying Party です。原文は多くの規定を ID Token に課す要件として書き、主体を書いていないため、値を決める規定の Actor を OpenID Provider とします。

### OIDCC-ISS-REQUIRED ID Token の `iss` の存在

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`REQUIRED. Issuer Identifier for the Issuer of the response.`
- **Level**：REQUIRED
- **Actor**：OpenID Provider
- **Condition**：ID Token である

### OIDCC-ISS-FORMAT ID Token の `iss` の形式

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`The iss value is a case-sensitive URL using the https scheme that contains scheme, host, and optionally, port number and path components and no query or fragment components.`
- **Level**：Definition
- **Condition**：ID Token である

§1.2 の Issuer Identifier の定義も同じ形式を定めています。RFC 7519 の `iss`（[RFC 7519 JSON Web Token](rfc7519.md) の RFC7519-ISS-FORMAT）は StringOrURI で、URL に限りません。

### OIDCC-SUB-REQUIRED ID Token の `sub` の存在

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`REQUIRED. Subject Identifier.`
- **Level**：REQUIRED
- **Actor**：OpenID Provider
- **Condition**：ID Token である

### OIDCC-SUB-IDENTIFIER ID Token の `sub` の一意性と再割当ての禁止

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`A locally unique and never reassigned identifier within the Issuer for the End-User, which is intended to be consumed by the Client, e.g., 24400320 or AItOawmwtWwcT0k51BayewNvutrJUqsvl6qs7A4.`
- **Level**：Definition
- **Condition**：ID Token である

原文は RFC 2119 の語を使わず、`sub` を Issuer の中で一意で、再割当てされない識別子と定めています。RFC 7519 の `sub`（[RFC 7519 JSON Web Token](rfc7519.md) の RFC7519-SUB-UNIQUE）は再割当てを禁じていません。

### OIDCC-SUB-LENGTH ID Token の `sub` の長さの上限

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`It MUST NOT exceed 255 ASCII [RFC20] characters in length.`
- **Level**：MUST NOT
- **Actor**：OpenID Provider
- **Condition**：ID Token である

### OIDCC-SUB-FORMAT ID Token の `sub` の値の形式

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`The sub value is a case-sensitive string.`
- **Level**：Definition
- **Condition**：ID Token である

### OIDCC-AUD-REQUIRED ID Token の `aud` の存在

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`REQUIRED. Audience(s) that this ID Token is intended for.`
- **Level**：REQUIRED
- **Actor**：OpenID Provider
- **Condition**：ID Token である

### OIDCC-AUD-CLIENT-ID ID Token の `aud` の `client_id`

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`It MUST contain the OAuth 2.0 client_id of the Relying Party as an audience value.`
- **Level**：MUST
- **Actor**：OpenID Provider
- **Condition**：ID Token である

`client_id` は、Relying Party が OpenID Provider に登録した OAuth 2.0 の Client の識別子です。

### OIDCC-AUD-OTHER ID Token の `aud` のほかの受信者

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`It MAY also contain identifiers for other audiences.`
- **Level**：MAY
- **Actor**：OpenID Provider
- **Condition**：ID Token である

Relying Party の側は、信頼しない受信者を含む ID Token を拒否します（OIDCC-CODE-AUD-UNTRUSTED）。

### OIDCC-AUD-FORMAT ID Token の `aud` の値の形式

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`In the general case, the aud value is an array of case-sensitive strings.`
- **Level**：Definition
- **Condition**：ID Token である

### OIDCC-AUD-SINGLE-STRING 受信者が一つの ID Token の `aud` の文字列表現

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`In the common special case when there is one audience, the aud value MAY be a single case-sensitive string.`
- **Level**：MAY
- **Actor**：OpenID Provider
- **Condition**：ID Token の受信者が一つである

この条項は、`aud` の値を配列とする OIDCC-AUD-FORMAT を緩めます。

### OIDCC-EXP-REQUIRED ID Token の `exp` の存在

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`REQUIRED. Expiration time on or after which the ID Token MUST NOT be accepted by the RP when performing authentication with the OP.`
- **Level**：REQUIRED
- **Actor**：OpenID Provider
- **Condition**：ID Token である

### OIDCC-EXP-REJECT ID Token の `exp` 以後の受理の禁止

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`Expiration time on or after which the ID Token MUST NOT be accepted by the RP when performing authentication with the OP. The processing of this parameter requires that the current date/time MUST be before the expiration date/time listed in the value.`
- **Level**：MUST NOT
- **Actor**：Relying Party
- **Condition**：ID Token で、Relying Party が OpenID Provider による認証を行う

2 つの文は、現在時刻が `exp` の時刻より前の ID Token だけを受理するという同じ規範を、受理しない側と受理する側から述べています。
OIDCC-EXP-LEEWAY が、この判定に猶予を認めます。

### OIDCC-EXP-LEEWAY ID Token の `exp` の判定の猶予

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`Implementers MAY provide for some small leeway, usually no more than a few minutes, to account for clock skew.`
- **Level**：MAY
- **Actor**：Implementer
- **Condition**：ID Token である

この条項は、OIDCC-EXP-REJECT と OIDCC-CODE-EXP を緩めます。猶予は通常は数分以内です（原文）。

### OIDCC-EXP-FORMAT ID Token の `exp` の値の形式

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`Its value is a JSON [RFC8259] number representing the number of seconds from 1970-01-01T00:00:00Z as measured in UTC until the date/time.`
- **Level**：Definition
- **Condition**：ID Token である

### OIDCC-IAT-REQUIRED ID Token の `iat` の存在

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`REQUIRED. Time at which the JWT was issued.`
- **Level**：REQUIRED
- **Actor**：OpenID Provider
- **Condition**：ID Token である

### OIDCC-IAT-FORMAT ID Token の `iat` の値の形式

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`Its value is a JSON number representing the number of seconds from 1970-01-01T00:00:00Z as measured in UTC until the date/time.`
- **Level**：Definition
- **Condition**：ID Token である

§3.1.3.7 の手順 10 は、`iat` を、現在時刻から離れすぎて発行された ID Token の拒否に使えるとし、許す範囲は Client ごとに異なるとしています（RFC 2119 の語を使っていません）。

### OIDCC-AUTH-TIME-REQUIRED `max_age` などで求められた `auth_time` の存在

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`When a max_age request is made or when auth_time is requested as an Essential Claim, then this Claim is REQUIRED; otherwise, its inclusion is OPTIONAL.`
- **Level**：REQUIRED
- **Actor**：OpenID Provider
- **Condition**：ID Token で、認証要求が `max_age` を指定するか、`auth_time` を Essential Claim として求める

`auth_time` は End-User の認証が行われた時刻です。

### OIDCC-AUTH-TIME-OPTIONAL `auth_time` の使用の任意性

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`When a max_age request is made or when auth_time is requested as an Essential Claim, then this Claim is REQUIRED; otherwise, its inclusion is OPTIONAL.`
- **Level**：OPTIONAL
- **Actor**：OpenID Provider
- **Condition**：ID Token で、認証要求が `max_age` を指定せず、`auth_time` を Essential Claim として求めない

### OIDCC-NONCE-VERIFY ID Token の `nonce` の照合

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`If present in the ID Token, Clients MUST verify that the nonce Claim Value is equal to the value of the nonce parameter sent in the Authentication Request.`
- **Level**：MUST
- **Actor**：Relying Party
- **Condition**：ID Token に `nonce` がある

`nonce` は、Client のセッションと ID Token を結び付け、リプレイ攻撃を緩和するための値で、認証要求から ID Token にそのまま渡されます（原文）。照合する値は Relying Party が認証要求で送った値なので、この条項は認証要求を前提とします。

### OIDCC-NONCE-INCLUDE 認証要求の `nonce` を ID Token に含めること

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`If present in the Authentication Request, Authorization Servers MUST include a nonce Claim in the ID Token with the Claim Value being the nonce value sent in the Authentication Request.`
- **Level**：MUST
- **Actor**：OpenID Provider
- **Condition**：ID Token で、認証要求に `nonce` がある

### OIDCC-NONCE-NO-PROCESSING `nonce` のほかの処理をしないこと

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`Authorization Servers SHOULD perform no other processing on nonce values used.`
- **Level**：SHOULD
- **Actor**：OpenID Provider

### OIDCC-NONCE-FORMAT ID Token の `nonce` の値の形式

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`The nonce value is a case-sensitive string.`
- **Level**：Definition
- **Condition**：ID Token である

### OIDCC-ACR-OPTIONAL `acr` の使用の任意性

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`OPTIONAL. Authentication Context Class Reference.`
- **Level**：OPTIONAL
- **Actor**：OpenID Provider
- **Condition**：ID Token である

`acr` は、行った認証が満たした Authentication Context Class を表す文字列で、値は大文字と小文字を区別します（原文）。

### OIDCC-ACR-LEVEL-0 `acr` の値 `0` の認証による金銭的価値のある資源への認可

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`Authentications with level 0 SHOULD NOT be used to authorize access to any resource of any monetary value.`
- **Level**：SHOULD NOT
- **Actor**：Relying Party
- **Condition**：ID Token の `acr` の値が `0` である

原文は受動態で主体を書いていません。ID Token の認証を資源への認可に使うのは Relying Party なので、Actor を Relying Party とします。

### OIDCC-ACR-VALUE-FORM `acr` の値の種類

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`An absolute URI or an RFC 6711 [RFC6711] registered name SHOULD be used as the acr value; registered names MUST NOT be used with a different meaning than that which is registered.`
- **Level**：SHOULD
- **Actor**：OpenID Provider
- **Condition**：ID Token に `acr` がある

### OIDCC-ACR-REGISTERED-MEANING 登録された `acr` の名前の意味

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`An absolute URI or an RFC 6711 [RFC6711] registered name SHOULD be used as the acr value; registered names MUST NOT be used with a different meaning than that which is registered.`
- **Level**：MUST NOT
- **Actor**：OpenID Provider
- **Condition**：ID Token の `acr` に登録された名前を使う

### OIDCC-AMR-OPTIONAL `amr` の使用の任意性

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`OPTIONAL. Authentication Methods References.`
- **Level**：OPTIONAL
- **Actor**：OpenID Provider
- **Condition**：ID Token である

`amr` は、認証に使った方式の識別子の、大文字と小文字を区別する文字列の配列です（原文）。

### OIDCC-AMR-REGISTERED `amr` の値の登録

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`Values used in the amr Claim SHOULD be from those registered in the IANA Authentication Method Reference Values registry [IANA.AMR] established by [RFC8176]; parties using this claim will need to agree upon the meanings of any unregistered values used, which may be context specific.`
- **Level**：SHOULD
- **Actor**：OpenID Provider
- **Condition**：ID Token に `amr` がある

### OIDCC-AZP-OPTIONAL `azp` の使用の任意性

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`OPTIONAL. Authorized party - the party to which the ID Token was issued.`
- **Level**：OPTIONAL
- **Actor**：OpenID Provider
- **Condition**：ID Token である

`azp` は、この仕様の範囲外の拡張を使う場合にだけ現れ、拡張を使わない実装は `azp` を使わず、現れても無視することが勧められています（原文は「are encouraged to」）。

### OIDCC-AZP-CLIENT-ID `azp` の `client_id`

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`If present, it MUST contain the OAuth 2.0 Client ID of this party.`
- **Level**：MUST
- **Actor**：OpenID Provider
- **Condition**：ID Token に `azp` がある

### OIDCC-CLAIM-OTHER ID Token のほかの Claim

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`ID Tokens MAY contain other Claims.`
- **Level**：MAY
- **Actor**：OpenID Provider
- **Condition**：ID Token である

### OIDCC-CLAIM-UNKNOWN-IGNORE ID Token の理解できない Claim の無視

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`Any Claims used that are not understood MUST be ignored.`
- **Level**：MUST
- **Actor**：Relying Party
- **Condition**：ID Token で、Relying Party がその Claim を理解しない

原文は受動態で主体を書いていません。ID Token を受け取って Claim を処理するのは Relying Party なので、Actor を Relying Party とします。

### OIDCC-ID-TOKEN-SIGNED ID Token の署名

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`ID Tokens MUST be signed using JWS [JWS] and optionally both signed and then encrypted using JWS [JWS] and JWE [JWE] respectively, thereby providing authentication, integrity, non-repudiation, and optionally, confidentiality, per Section 16.14.`
- **Level**：MUST
- **Actor**：OpenID Provider
- **Condition**：ID Token である

原文は受動態で主体を書いていません。ID Token に署名するのは OpenID Provider なので、Actor を OpenID Provider とします。
`alg` に `none` を使える場合は OIDCC-ALG-NONE が定めます。

### OIDCC-ALG-NONE ID Token の `none` の禁止

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`ID Tokens MUST NOT use none as the alg value unless the Response Type used returns no ID Token from the Authorization Endpoint (such as when using the Authorization Code Flow) and the Client explicitly requested the use of none at Registration time.`
- **Level**：MUST NOT
- **Actor**：OpenID Provider
- **Condition**：ID Token で、Response Type が Authorization Endpoint から ID Token を返すか、Client が登録時に `none` の使用を明示的に求めていない

### OIDCC-KEY-HEADER-AVOID ID Token の鍵を参照・運ぶ Header Parameter

- **Source**：[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)
- **Quote**：`ID Tokens SHOULD NOT use the JWS or JWE x5u, x5c, jku, or jwk Header Parameter fields.`
- **Level**：SHOULD NOT
- **Actor**：OpenID Provider
- **Condition**：ID Token である

使う鍵の参照は、Discovery と登録のパラメーターで事前に伝えます（原文）。

## 2. ID Token Validation in the Authorization Code Flow

OpenID Connect Core §3.1.3.7 は、Authorization Code Flow で Token Response に含まれる ID Token を Client が検証する手順を定めています。手順の多くの文は主語を「The Client」と書き、主体を書かない文（`iss`、TLS、`alg`、`exp`、`nonce` の手順）も手順を行うのは Client なので、この節の条項の Actor を Relying Party とします。
手順 1（暗号化された ID Token の復号）は JWE を扱うため対象外です。手順 8 は、`alg` が HMAC を使うアルゴリズムの場合に、`aud` の `client_id` に対応する `client_secret` の UTF-8 表現を検証の鍵に使うとし、`aud` が複数の値を持つ場合の振る舞いを定めていません（RFC 2119 の語を使っていません）。

### OIDCC-CODE-VALIDATE Authorization Code Flow の ID Token の検証

- **Source**：[OpenID Connect Core 1.0 §3.1.3.7](https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation)
- **Quote**：`Clients MUST validate the ID Token in the Token Response in the following manner:`
- **Level**：MUST
- **Actor**：Relying Party
- **Condition**：Authorization Code Flow の Token Response に含まれる ID Token である

個々の手順の規範は、この節の残りの条項が定めます。

### OIDCC-CODE-ISS-MATCH Issuer Identifier と `iss` の完全一致

- **Source**：[OpenID Connect Core 1.0 §3.1.3.7](https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation)
- **Quote**：`The Issuer Identifier for the OpenID Provider (which is typically obtained during Discovery) MUST exactly match the value of the iss (issuer) Claim.`
- **Level**：MUST
- **Actor**：Relying Party
- **Condition**：Authorization Code Flow の Token Response に含まれる ID Token である

完全一致の比較は §14 の規則（OIDCC-STRING-UNESCAPE、OIDCC-STRING-NO-NORMALIZATION、OIDCC-STRING-CODE-POINT）に従います。

### OIDCC-CODE-AUD-CLIENT `aud` が自身の `client_id` を含むことの検証

- **Source**：[OpenID Connect Core 1.0 §3.1.3.7](https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation)
- **Quote**：`The Client MUST validate that the aud (audience) Claim contains its client_id value registered at the Issuer identified by the iss (issuer) Claim as an audience. The aud (audience) Claim MAY contain an array with more than one element. The ID Token MUST be rejected if the ID Token does not list the Client as a valid audience, or if it contains additional audiences not trusted by the Client.`
- **Level**：MUST
- **Actor**：Relying Party
- **Condition**：Authorization Code Flow の Token Response に含まれる ID Token である

1 つ目の文の検証と、3 つ目の文のうち「does not list the Client as a valid audience」の拒否は、一つの受理の判定を表すため、一つの条項にします。3 つ目の文のうち信頼しない受信者による拒否は OIDCC-CODE-AUD-UNTRUSTED、2 つ目の文は OIDCC-CODE-AUD-MULTIPLE です。

### OIDCC-CODE-AUD-MULTIPLE `aud` の複数の要素

- **Source**：[OpenID Connect Core 1.0 §3.1.3.7](https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation)
- **Quote**：`The aud (audience) Claim MAY contain an array with more than one element.`
- **Level**：MAY
- **Actor**：Relying Party
- **Condition**：Authorization Code Flow の Token Response に含まれる ID Token である

原文は `aud` に許す形として書いています。検証の手順の中の文なので、Relying Party が、要素が複数の配列の `aud` を、それだけを理由に拒否しなくてよいことを表すと読み、Actor を Relying Party とします。値を作る側の許可は OIDCC-AUD-OTHER です。

### OIDCC-CODE-AUD-UNTRUSTED 信頼しない受信者を含む ID Token の拒否

- **Source**：[OpenID Connect Core 1.0 §3.1.3.7](https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation)
- **Quote**：`The ID Token MUST be rejected if the ID Token does not list the Client as a valid audience, or if it contains additional audiences not trusted by the Client.`
- **Level**：MUST
- **Actor**：Relying Party
- **Condition**：Authorization Code Flow の Token Response に含まれる ID Token で、`aud` が Client の信頼しない受信者を含む

どの受信者を信頼するかは、原文が定めず、Client が決めます。

### OIDCC-CODE-AZP-EXTENSION 拡張による `azp` の検証

- **Source**：[OpenID Connect Core 1.0 §3.1.3.7](https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation)
- **Quote**：`If the implementation is using extensions (which are beyond the scope of this specification) that result in the azp (authorized party) Claim being present, it SHOULD validate the azp value as specified by those extensions.`
- **Level**：SHOULD
- **Actor**：Relying Party
- **Condition**：Authorization Code Flow の Token Response に含まれる ID Token で、Relying Party が `azp` を含める拡張を使う

### OIDCC-CODE-AZP-CLIENT-ID `azp` と自身の `client_id` の照合

- **Source**：[OpenID Connect Core 1.0 §3.1.3.7](https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation)
- **Quote**：`This validation MAY include that when an azp (authorized party) Claim is present, the Client SHOULD verify that its client_id is the Claim Value.`
- **Level**：MAY
- **Actor**：Relying Party
- **Condition**：Authorization Code Flow の Token Response に含まれる ID Token に `azp` がある

原文は、OIDCC-CODE-AZP-EXTENSION の検証がこの照合を含んでもよい（MAY）とし、含める場合の照合を SHOULD で述べています。照合を含めるかどうかの選択が外側の規範なので、Level を MAY とします。

### OIDCC-CODE-TLS-ISSUER TLS のサーバーの検証による Issuer の検証

- **Source**：[OpenID Connect Core 1.0 §3.1.3.7](https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation)
- **Quote**：`If the ID Token is received via direct communication between the Client and the Token Endpoint (which it is in this flow), the TLS server validation MAY be used to validate the issuer in place of checking the token signature.`
- **Level**：MAY
- **Actor**：Relying Party
- **Condition**：ID Token を、Client と Token Endpoint の直接の通信で受け取る

Authorization Code Flow では、ID Token を Token Endpoint から直接受け取るため、この条件に当たります（原文）。この条項は、署名の検証を求める OIDCC-CODE-SIGNATURE-VALIDATE が対象としない ID Token を定めます。

### OIDCC-CODE-SIGNATURE-VALIDATE 直接の通信以外で受け取った ID Token の署名の検証

- **Source**：[OpenID Connect Core 1.0 §3.1.3.7](https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation)
- **Quote**：`The Client MUST validate the signature of all other ID Tokens according to JWS [JWS] using the algorithm specified in the JWT alg Header Parameter.`
- **Level**：MUST
- **Actor**：Relying Party
- **Condition**：ID Token を、Client と Token Endpoint の直接の通信以外で受け取る

「all other ID Tokens」は、OIDCC-CODE-TLS-ISSUER の条件に当たらない ID Token です。Authorization Code Flow の ID Token は Token Endpoint から直接受け取るため、この文が意味を持つのは、§3.1.3.7 の手順をほかのフロー（Hybrid Flow など）に当てはめる場合です。このため、この条項と OIDCC-CODE-TLS-ISSUER の Condition は、Authorization Code Flow に限らず、受け取り方で書きます。

### OIDCC-CODE-ISSUER-KEYS Issuer が提供する鍵の使用

- **Source**：[OpenID Connect Core 1.0 §3.1.3.7](https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation)
- **Quote**：`The Client MUST use the keys provided by the Issuer.`
- **Level**：MUST
- **Actor**：Relying Party
- **Condition**：Authorization Code Flow の Token Response に含まれる ID Token である

この文は署名の検証を求める文に続き、検証に使う鍵を定めています。

### OIDCC-CODE-ALG-DEFAULT ID Token の `alg` の既定値

- **Source**：[OpenID Connect Core 1.0 §3.1.3.7](https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation)
- **Quote**：`The alg value SHOULD be the default of RS256 or the algorithm sent by the Client in the id_token_signed_response_alg parameter during Registration.`
- **Level**：SHOULD
- **Actor**：Relying Party
- **Condition**：Authorization Code Flow の Token Response に含まれる ID Token である

### OIDCC-CODE-EXP 現在時刻が `exp` より前であることの検証

- **Source**：[OpenID Connect Core 1.0 §3.1.3.7](https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation)
- **Quote**：`The current time MUST be before the time represented by the exp Claim.`
- **Level**：MUST
- **Actor**：Relying Party
- **Condition**：Authorization Code Flow の Token Response に含まれる ID Token である

OIDCC-EXP-LEEWAY が、この判定に猶予を認めます。

### OIDCC-CODE-NONCE 認証要求で送った `nonce` の照合

- **Source**：[OpenID Connect Core 1.0 §3.1.3.7](https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation)
- **Quote**：`If a nonce value was sent in the Authentication Request, a nonce Claim MUST be present and its value checked to verify that it is the same value as the one that was sent in the Authentication Request.`
- **Level**：MUST
- **Actor**：Relying Party
- **Condition**：Authorization Code Flow の Token Response に含まれる ID Token で、認証要求で `nonce` を送った

### OIDCC-CODE-NONCE-REPLAY `nonce` によるリプレイ攻撃の確認

- **Source**：[OpenID Connect Core 1.0 §3.1.3.7](https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation)
- **Quote**：`The Client SHOULD check the nonce value for replay attacks.`
- **Level**：SHOULD
- **Actor**：Relying Party
- **Condition**：Authorization Code Flow の Token Response に含まれる ID Token で、認証要求で `nonce` を送った

リプレイ攻撃を検出する方法は Client ごとに異なります（原文）。

### OIDCC-CODE-ACR 求めた `acr` の確認

- **Source**：[OpenID Connect Core 1.0 §3.1.3.7](https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation)
- **Quote**：`If the acr Claim was requested, the Client SHOULD check that the asserted Claim Value is appropriate.`
- **Level**：SHOULD
- **Actor**：Relying Party
- **Condition**：Authorization Code Flow の Token Response に含まれる ID Token で、`acr` を求めた

### OIDCC-CODE-AUTH-TIME 求めた `auth_time` の確認

- **Source**：[OpenID Connect Core 1.0 §3.1.3.7](https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation)
- **Quote**：`If the auth_time Claim was requested, either through a specific request for this Claim or by using the max_age parameter, the Client SHOULD check the auth_time Claim value and request re-authentication if it determines too much time has elapsed since the last End-User authentication.`
- **Level**：SHOULD
- **Actor**：Relying Party
- **Condition**：Authorization Code Flow の Token Response に含まれる ID Token で、`auth_time` を求めた

## 3. ID Token Validation in the Implicit and Hybrid Flows

Implicit Flow では、すべてのトークンを Authorization Endpoint から返し、Token Endpoint を使いません。Hybrid Flow では、ID Token を Authorization Endpoint と Token Endpoint のどちらからも返し得ます。

### OIDCC-IMPLICIT-VALIDATE Implicit Flow の ID Token の検証

- **Source**：[OpenID Connect Core 1.0 §3.2.2.11](https://openid.net/specs/openid-connect-core-1_0.html#ImplicitIDTValidation)
- **Quote**：`When using the Implicit Flow, the contents of the ID Token MUST be validated in the same manner as for the Authorization Code Flow, as defined in Section 3.1.3.7, with the exception of the differences specified in this section.`
- **Level**：MUST
- **Actor**：Relying Party
- **Condition**：Implicit Flow の ID Token である

原文は受動態で主体を書いていません。§3.2.2.11 の手順の主語が「The Client」なので、Actor を Relying Party とします。この節の残りの条項も同じです。
差分は、署名の検証（OIDCC-IMPLICIT-SIGNATURE）と `nonce` の照合（OIDCC-IMPLICIT-NONCE、OIDCC-IMPLICIT-NONCE-REPLAY）です。

### OIDCC-IMPLICIT-SIGNATURE Implicit Flow の ID Token の署名の検証

- **Source**：[OpenID Connect Core 1.0 §3.2.2.11](https://openid.net/specs/openid-connect-core-1_0.html#ImplicitIDTValidation)
- **Quote**：`The Client MUST validate the signature of the ID Token according to JWS [JWS] using the algorithm specified in the alg Header Parameter of the JOSE Header.`
- **Level**：MUST
- **Actor**：Relying Party
- **Condition**：Implicit Flow の ID Token である

Implicit Flow の ID Token は Token Endpoint から直接受け取らないため、TLS のサーバーの検証で署名の検証に代えられません（OIDCC-CODE-TLS-ISSUER の条件に当たりません）。

### OIDCC-IMPLICIT-NONCE Implicit Flow の `nonce` の照合

- **Source**：[OpenID Connect Core 1.0 §3.2.2.11](https://openid.net/specs/openid-connect-core-1_0.html#ImplicitIDTValidation)
- **Quote**：`The value of the nonce Claim MUST be checked to verify that it is the same value as the one that was sent in the Authentication Request.`
- **Level**：MUST
- **Actor**：Relying Party
- **Condition**：Implicit Flow の ID Token である

Implicit Flow の認証要求では `nonce` が REQUIRED なので（§3.2.2.1）、Authorization Code Flow（OIDCC-CODE-NONCE）と異なり、`nonce` を送った場合という条件がありません。

### OIDCC-IMPLICIT-NONCE-REPLAY Implicit Flow の `nonce` によるリプレイ攻撃の確認

- **Source**：[OpenID Connect Core 1.0 §3.2.2.11](https://openid.net/specs/openid-connect-core-1_0.html#ImplicitIDTValidation)
- **Quote**：`The Client SHOULD check the nonce value for replay attacks.`
- **Level**：SHOULD
- **Actor**：Relying Party
- **Condition**：Implicit Flow の ID Token である

### OIDCC-HYBRID-AUTHORIZATION-ENDPOINT Hybrid Flow で Authorization Endpoint から返る ID Token の検証

- **Source**：[OpenID Connect Core 1.0 §3.3.2.12](https://openid.net/specs/openid-connect-core-1_0.html#HybridIDTValidation)
- **Quote**：`When using the Hybrid Flow, the contents of an ID Token returned from the Authorization Endpoint MUST be validated in the same manner as for the Implicit Flow, as defined in Section 3.2.2.11.`
- **Level**：MUST
- **Actor**：Relying Party
- **Condition**：Hybrid Flow で Authorization Endpoint から返る ID Token である

原文は受動態で主体を書いていません。ID Token を検証するのは Client なので、Actor を Relying Party とします。次の条項も同じです。

### OIDCC-HYBRID-TOKEN-ENDPOINT Hybrid Flow で Token Endpoint から返る ID Token の検証

- **Source**：[OpenID Connect Core 1.0 §3.3.3.7](https://openid.net/specs/openid-connect-core-1_0.html#HybridIDTValidation2)
- **Quote**：`When using the Hybrid Flow, the contents of an ID Token returned from the Token Endpoint MUST be validated in the same manner as for the Authorization Code Flow, as defined in Section 3.1.3.7.`
- **Level**：MUST
- **Actor**：Relying Party
- **Condition**：Hybrid Flow で Token Endpoint から返る ID Token である

## 4. Signing

OpenID Connect Core §10.1 の署名の規定は、ID Token に限らず、UserInfo Response、Request Object、Client 認証の JWT にも適用されます（§10）。原文は署名する主体を「The signing party」「the sender」「The signer」と呼ぶため、Actor を Signer に統一します。

### OIDCC-SIGNING-ALG-SELECT 受信者が対応するアルゴリズムからの署名のアルゴリズムの選択

- **Source**：[OpenID Connect Core 1.0 §10.1](https://openid.net/specs/openid-connect-core-1_0.html#Signing)
- **Quote**：`The signing party MUST select a signature algorithm based on the algorithms supported by the recipient.`
- **Level**：MUST
- **Actor**：Signer

### OIDCC-SIGNING-ASYMMETRIC-ALG RSA と ECDSA の署名の `alg`

- **Source**：[OpenID Connect Core 1.0 §10.1](https://openid.net/specs/openid-connect-core-1_0.html#Signing)
- **Quote**：`When using RSA or ECDSA Signatures, the alg Header Parameter value of the JOSE Header MUST be set to an appropriate algorithm as defined in JSON Web Algorithms [JWA].`
- **Level**：MUST
- **Actor**：Signer
- **Condition**：RSA か ECDSA の署名を使う

原文は受動態で主体を書いていません。JOSE Header の `alg` を設定するのは Signer なので、Actor を Signer とします。この節の残りの受動態の条項も同じです。

### OIDCC-SIGNING-KEY-PUBLISHED 署名の秘密鍵と公開した JWK Set の公開鍵の対応

- **Source**：[OpenID Connect Core 1.0 §10.1](https://openid.net/specs/openid-connect-core-1_0.html#Signing)
- **Quote**：`The private key used to sign the content MUST be associated with a public key used for signature verification published by the sender in its JWK Set document.`
- **Level**：MUST
- **Actor**：Signer
- **Condition**：RSA か ECDSA の署名を使う

### OIDCC-SIGNING-KID 複数の鍵がある JWK Set での `kid`

- **Source**：[OpenID Connect Core 1.0 §10.1](https://openid.net/specs/openid-connect-core-1_0.html#Signing)
- **Quote**：`If there are multiple keys in the referenced JWK Set document, a kid value MUST be provided in the JOSE Header.`
- **Level**：MUST
- **Actor**：Signer
- **Condition**：RSA か ECDSA の署名を使い、参照する JWK Set に複数の鍵がある

RFC 7515 の `kid` は OPTIONAL（[RFC 7515 JSON Web Signature](rfc7515.md) の RFC7515-KID-OPTIONAL）ですが、この条件では OpenID Connect Core が `kid` を求めます。

### OIDCC-SIGNING-KEY-USAGE 署名に対応する鍵の用途

- **Source**：[OpenID Connect Core 1.0 §10.1](https://openid.net/specs/openid-connect-core-1_0.html#Signing)
- **Quote**：`The key usage of the respective keys MUST support signing.`
- **Level**：MUST
- **Actor**：Signer
- **Condition**：RSA か ECDSA の署名を使う

鍵の用途は、JWK の `use` か `key_ops` で表します（[RFC 7517 JSON Web Key](rfc7517.md) の RFC7517-USE-FORMAT、RFC7517-KEY-OPS-FORMAT）。

### OIDCC-SIGNING-MAC-ALG MAC による署名の `alg`

- **Source**：[OpenID Connect Core 1.0 §10.1](https://openid.net/specs/openid-connect-core-1_0.html#Signing)
- **Quote**：`When using MAC-based signatures, the alg Header Parameter value of the JOSE Header MUST be set to a MAC algorithm, as defined in JSON Web Algorithms [JWA].`
- **Level**：MUST
- **Actor**：Signer
- **Condition**：MAC による署名を使う

MAC の鍵は、`client_secret` の値の UTF-8 表現のオクテット列です（原文）。

### OIDCC-SIGNING-MAC-PUBLIC-CLIENT 公開 Client による MAC の署名の禁止

- **Source**：[OpenID Connect Core 1.0 §10.1](https://openid.net/specs/openid-connect-core-1_0.html#Signing)
- **Quote**：`Symmetric signatures MUST NOT be used by public (non-confidential) Clients because of their inability to keep secrets.`
- **Level**：MUST NOT
- **Actor**：Relying Party
- **Condition**：Relying Party が公開 Client（機密でない Client）である

### OIDCC-KEY-ROTATION-RETAIN 退役させた署名鍵の JWK Set での保持

- **Source**：[OpenID Connect Core 1.0 §10.1.1](https://openid.net/specs/openid-connect-core-1_0.html#RotateSigKeys)
- **Quote**：`The JWK Set document at the jwks_uri SHOULD retain recently decommissioned signing keys for a reasonable period of time to facilitate a smooth transition.`
- **Level**：SHOULD
- **Actor**：Signer

原文は JWK Set に課す推奨として書き、主体を書いていません。§10.1.1 は、`jwks_uri` に JWK Set を公開する主体を「The signer」と呼ぶため、Actor を Signer とします。
§10.1.1 の見出しは「Rotation of Asymmetric Signing Keys」で、この推奨は非対称鍵の署名鍵の更新を扱います。
§10.1.1 は鍵の更新の方法として、Signer が JWK Set に新しい鍵を加えて `kid` で切り替えを伝え、検証する側は知らない `kid` を見たら `jwks_uri` から鍵を取得し直すことを述べています（RFC 2119 の語を使っていません）。

## 5. String Operations

OpenID Connect Core §14 は、Unicode の文字列の比較がセキュリティに大きく影響するとし、JSON の文字列とほかの Unicode の文字列の比較の手順を定めています。原文は比較の主体を書いていません。OpenID Connect のメッセージを処理する Relying Party と OpenID Provider の両方が比較を行い、§2 の `azp` の説明は処理する主体を「implementations」と呼ぶため、この節の条項の Actor を Implementation とします。

### OIDCC-STRING-UNESCAPE 比較の前の JSON のエスケープの除去

- **Source**：[OpenID Connect Core 1.0 §14](https://openid.net/specs/openid-connect-core-1_0.html#StringOps)
- **Quote**：`Therefore, comparisons between JSON strings and other Unicode strings MUST be performed as specified below: Remove any JSON applied escaping to produce an array of Unicode code points.`
- **Level**：MUST
- **Actor**：Implementation

1 つ目の文の MUST は、手順全体に従うことを求めます。手順 2 と手順 3 は、それぞれ OIDCC-STRING-NO-NORMALIZATION と OIDCC-STRING-CODE-POINT が定めます。

### OIDCC-STRING-NO-NORMALIZATION Unicode 正規化の禁止

- **Source**：[OpenID Connect Core 1.0 §14](https://openid.net/specs/openid-connect-core-1_0.html#StringOps)
- **Quote**：`Unicode Normalization [USA15] MUST NOT be applied at any point to either the JSON string or to the string it is to be compared against.`
- **Level**：MUST NOT
- **Actor**：Implementation

### OIDCC-STRING-CODE-POINT コードポイント単位の等価比較

- **Source**：[OpenID Connect Core 1.0 §14](https://openid.net/specs/openid-connect-core-1_0.html#StringOps)
- **Quote**：`Comparisons between the two strings MUST be performed as a Unicode code point to code point equality comparison.`
- **Level**：MUST
- **Actor**：Implementation

RFC 7159 §8.3 の比較（[RFC 7159 JSON](rfc7159.md) の RFC7159-STRING-COMPARE）はコードユニット単位で、この条項はコードポイント単位です。エスケープを解いた結果が有効な Unicode の文字列なら、2 つの比較の等しいかどうかの判定は一致します。

### OIDCC-SPACE-DELIMITER 空白区切りの文字列の区切り文字

- **Source**：[OpenID Connect Core 1.0 §14](https://openid.net/specs/openid-connect-core-1_0.html#StringOps)
- **Quote**：`In several places, this specification uses space-delimited lists of strings. In all such cases, a single ASCII space character (0x20) MUST be used as the delimiter.`
- **Level**：MUST
- **Actor**：Implementation

## 6. References

- [RFC 2119 — Key words for use in RFCs to Indicate Requirement Levels](https://www.rfc-editor.org/rfc/rfc2119.html)（1997）
- [OpenID Connect Core 1.0 incorporating errata set 2](https://openid.net/specs/openid-connect-core-1_0.html)（2023、§1.2、§2、§3.1.3.7、§3.2.2.1、§3.2.2.11、§3.3.2.12、§3.3.3.7、§10、§10.1、§10.1.1、§14、§15）
