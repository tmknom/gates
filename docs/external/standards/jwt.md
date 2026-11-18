# JWT Standards

この文書は、JSON、JWT、JWT の Best Current Practices の標準仕様から、Claims の検証と JWT の発行・検証の考慮事項に関係する規定を、ID の付いた条項に分けて整理したものです。標準仕様の代替ではなく、規定の適用対象と条件はリンク先の原文を優先します。

- **対象**：RFC 7159 の文字列の比較（§8.3）、RFC 7519 の StringOrURI と NumericDate の定義（§2）、JWT Claims Set と登録済みの Claim（§4、§4.1）、文字列の比較（§7.3）、RFC 8725 の JWT の種類の取り違えへの対策（§2.8）と Best Practices（§3）
- **対象外**：OAuth 2.0 の認可フロー全体、JWE による暗号化（RFC 8725 §3.2 の RSA-PKCS1 v1.5 の回避、§3.4、§3.6 を含む）、暗号鍵の生成と暗号アルゴリズムの実装（RFC 8725 §3.2 の ECDSA の実装、§3.5）、RFC 8725 §2 の脅威の説明（§2.8 の MUST は条項にする）、Claim 名の登録と名前空間（RFC 7519 §4.2、§4.3）、JWT の概要（RFC 7519 §3）、JOSE Header と JWT の作成・検証の手順（RFC 7519 §5–§7.2）、実装の要件（RFC 7519 §8）、URI、IANA、セキュリティとプライバシーの考慮事項（RFC 7519 §9–§12）、OpenID Connect が ID Token に追加する規定

各条項の Level は、原文が使う [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119.html) の語か、RFC 2119 の語を使わない定義を表す Definition です。
RFC 7519 と RFC 8725 は、RFC 2119 の語をすべて大文字で書いた場合に限り、その意味で使います。小文字の「should」「may」の文は条項にしていません。RFC 7159 はこの限定を書いていませんが、条項にした §8.3 は小文字の RFC 2119 の語を含みません。

## 1. RFC 7159 JSON

### String Comparison

#### RFC7159-STRING-COMPARE コードユニット単位の文字列比較

- **Source**：[RFC 7159 §8.3](https://www.rfc-editor.org/rfc/rfc7159.html#section-8.3)
- **Quote**：`Implementations that transform the textual representation into sequences of Unicode code units and then perform the comparison numerically, code unit by code unit, are interoperable in the sense that implementations will agree in all cases on equality or inequality of two strings.`
- **Level**：Definition

原文は RFC 2119 の語を使わず、エスケープを解いて Unicode のコードユニットの列にしてから比較する実装どうしは、等しいかどうかの判定が一致すると述べています。
RFC 7519 §7.3 が JWT の文字列比較にこの規則を使うことを MUST とするため（RFC7519-STRING-COMPARE）、Definition とします。
エスケープを解かずに比較する実装は、同じ文字を異なるエスケープで書いた文字列（バックスラッシュを `\\` で書いたものと、`\u` と 16 進数の符号位置で書いたもの）を、異なる文字列と判定し得ます。

## 2. RFC 7519 JSON Web Token

### StringOrURI and NumericDate

#### RFC7519-STRINGORURI-ARBITRARY StringOrURI に任意の文字列を使うこと

- **Source**：[RFC 7519 §2](https://www.rfc-editor.org/rfc/rfc7519.html#section-2)
- **Quote**：`A JSON string value, with the additional requirement that while arbitrary string values MAY be used, any value containing a ":" character MUST be a URI [RFC3986].`
- **Level**：MAY
- **Actor**：Issuer

原文は StringOrURI の定義の中で値の要件を述べ、主体を書いていません。StringOrURI の値を JWT に入れるのは Issuer なので、Actor を Issuer とします。
`:` を含む値は、RFC7519-STRINGORURI-URI が URI に限ります。

#### RFC7519-STRINGORURI-URI `:` を含む StringOrURI の URI への限定

- **Source**：[RFC 7519 §2](https://www.rfc-editor.org/rfc/rfc7519.html#section-2)
- **Quote**：`A JSON string value, with the additional requirement that while arbitrary string values MAY be used, any value containing a ":" character MUST be a URI [RFC3986].`
- **Level**：MUST
- **Actor**：Issuer
- **Condition**：値が `:` を含む

原文は StringOrURI の定義の中で値の要件を述べ、主体を書いていません。StringOrURI の値を JWT に入れるのは Issuer なので、Actor を Issuer とします。

#### RFC7519-STRINGORURI-COMPARE StringOrURI の大文字小文字を区別する比較

- **Source**：[RFC 7519 §2](https://www.rfc-editor.org/rfc/rfc7519.html#section-2)
- **Quote**：`StringOrURI values are compared as case-sensitive strings with no transformations or canonicalizations applied.`
- **Level**：Definition

URI として正規化すれば等しくなる値（スキームやホストの大文字と小文字だけが異なる値など）も、異なる値として比較します。

#### RFC7519-NUMERICDATE-FORMAT NumericDate の形式

- **Source**：[RFC 7519 §2](https://www.rfc-editor.org/rfc/rfc7519.html#section-2)
- **Quote**：`A JSON numeric value representing the number of seconds from 1970-01-01T00:00:00Z UTC until the specified UTC date/time, ignoring leap seconds. This is equivalent to the IEEE Std 1003.1, 2013 Edition [POSIX.1] definition "Seconds Since the Epoch", in which each day is accounted for by exactly 86400 seconds, other than that non-integer values can be represented.`
- **Level**：Definition

NumericDate は JSON の数値で、整数に限りません。数字を JSON の文字列で表した値（`"1700000000"`）は NumericDate ではありません。

### JWT Claims Set

RFC 7519 §4 は、JWT が有効とみなされるために含まなければならない Claim の集合は文脈に依存し、この仕様の範囲外であるとしています。
§4.1 は、登録済みの Claim のどれも、すべての場合に使用や実装を必須とするものではないとし、JWT を使うアプリケーションが、どの Claim を使い、いつ必須または任意とするかを定めるべきだとしています（小文字の「should」）。

#### RFC7519-CLAIM-NAME-UNIQUE Claim 名の一意性

- **Source**：[RFC 7519 §4](https://www.rfc-editor.org/rfc/rfc7519.html#section-4)
- **Quote**：`The Claim Names within a JWT Claims Set MUST be unique; JWT parsers MUST either reject JWTs with duplicate Claim Names or use a JSON parser that returns only the lexically last duplicate member name, as specified in Section 15.12 ("The JSON Object") of ECMAScript 5.1 [ECMAScript].`
- **Level**：MUST
- **Actor**：Issuer

原文は Claims Set に課す要件として書き、主体を書いていません。JWT Claims Set を作るのは Issuer なので、Actor を Issuer とします。
重複した Claim 名を受け取ったときの処理は、RFC7519-CLAIM-NAME-DUPLICATE が定めます。

#### RFC7519-CLAIM-NAME-DUPLICATE 重複した Claim 名の処理

- **Source**：[RFC 7519 §4](https://www.rfc-editor.org/rfc/rfc7519.html#section-4)
- **Quote**：`The Claim Names within a JWT Claims Set MUST be unique; JWT parsers MUST either reject JWTs with duplicate Claim Names or use a JSON parser that returns only the lexically last duplicate member name, as specified in Section 15.12 ("The JSON Object") of ECMAScript 5.1 [ECMAScript].`
- **Level**：MUST
- **Actor**：JWT Parser

JWT Parser は、重複した Claim 名を持つ JWT を拒否するか、同じ名前のメンバーのうち字句の上で最後のものだけを返す JSON パーサーを使うか、のどちらかを選びます。
重複を拒否しない JWT Parser では、最後の値が採用されます。

#### RFC7519-CLAIM-UNKNOWN-IGNORE 理解できない Claim の無視

- **Source**：[RFC 7519 §4](https://www.rfc-editor.org/rfc/rfc7519.html#section-4)
- **Quote**：`Specific applications of JWTs will require implementations to understand and process some claims in particular ways. However, in the absence of such requirements, all claims that are not understood by implementations MUST be ignored.`
- **Level**：MUST
- **Actor**：Implementation
- **Condition**：JWT を使うアプリケーションが、その Claim を理解して特定の方法で処理することを要求していない

### `iss`

#### RFC7519-ISS-FORMAT `iss` の値の形式

- **Source**：[RFC 7519 §4.1.1](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.1)
- **Quote**：`The "iss" value is a case-sensitive string containing a StringOrURI value.`
- **Level**：Definition

`iss` の処理は、一般にアプリケーション固有です（原文）。

#### RFC7519-ISS-OPTIONAL `iss` の使用の任意性

- **Source**：[RFC 7519 §4.1.1](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.1)
- **Quote**：`Use of this claim is OPTIONAL.`
- **Level**：OPTIONAL
- **Actor**：Issuer

原文は使用の主体を書いていません。Claim を JWT に含めるかを決めるのは Issuer なので、Actor を Issuer とします。

### `sub`

#### RFC7519-SUB-UNIQUE `sub` の一意性

- **Source**：[RFC 7519 §4.1.2](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.2)
- **Quote**：`The subject value MUST either be scoped to be locally unique in the context of the issuer or be globally unique.`
- **Level**：MUST
- **Actor**：Issuer

原文は値に課す要件として書き、主体を書いていません。`sub` の値を割り当てるのは Issuer なので、Actor を Issuer とします。
一意である範囲は、Issuer の中か、全体のどちらでもかまいません。

#### RFC7519-SUB-FORMAT `sub` の値の形式

- **Source**：[RFC 7519 §4.1.2](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.2)
- **Quote**：`The "sub" value is a case-sensitive string containing a StringOrURI value.`
- **Level**：Definition

`sub` の処理は、一般にアプリケーション固有です（原文）。

#### RFC7519-SUB-OPTIONAL `sub` の使用の任意性

- **Source**：[RFC 7519 §4.1.2](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.2)
- **Quote**：`Use of this claim is OPTIONAL.`
- **Level**：OPTIONAL
- **Actor**：Issuer

原文は使用の主体を書いていません。Claim を JWT に含めるかを決めるのは Issuer なので、Actor を Issuer とします。

### `aud`

#### RFC7519-AUD-INTENDED 処理を意図された受信者による `aud` の中の自身の識別

- **Source**：[RFC 7519 §4.1.3](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.3)
- **Quote**：`Each principal intended to process the JWT MUST identify itself with a value in the audience claim.`
- **Level**：MUST
- **Actor**：Recipient

原文の主語は JWT の処理を意図された主体で、§4.1.3 はこの主体を「recipients」と呼ぶため、Actor を Recipient とします。
「identify itself with a value」は、次の文（RFC7519-AUD-REJECT）でも、受信者が `aud` の値の中に自身を識別する値を見つける行為を指します。
この文は `aud` が存在する条件を書いていませんが、`aud` の使用は OPTIONAL なので（RFC7519-AUD-OPTIONAL）、この条項は `aud` を含む JWT に適用されます。
受信者が自身を識別できるのは、Issuer が受信者を識別する値を `aud` に入れた場合に限られます。ただしこれはこの条項から導かれる帰結であり、Issuer に課す規範として原文が書いたものではありません。
RFC7519-AUD-REJECT は、処理を意図された受信者に限らず、`aud` を処理するすべての主体に、`aud` が存在する条件のもとで拒否を求めます。対象と条件が異なるため、2 つの条項に分けます。

#### RFC7519-AUD-REJECT 受信者を含まない aud の拒否

- **Source**：[RFC 7519 §4.1.3](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.3)
- **Quote**：`If the principal processing the claim does not identify itself with a value in the "aud" claim when this claim is present, then the JWT MUST be rejected.`
- **Level**：MUST
- **Actor**：Recipient
- **Condition**：`aud` が存在する

原文は受動態で、主体を「the principal processing the claim」と書いています。§4.1.3 は `aud` が識別する主体を「recipients」と呼ぶため、Actor を Recipient とします。
`aud` の値の解釈（受信者が、どの値を自身を識別する値とするか）は、一般にアプリケーション固有です（原文）。

#### RFC7519-AUD-FORMAT `aud` の値の形式

- **Source**：[RFC 7519 §4.1.3](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.3)
- **Quote**：`In the general case, the "aud" value is an array of case-sensitive strings, each containing a StringOrURI value.`
- **Level**：Definition

受信者が一つの JWT では、RFC7519-AUD-SINGLE-STRING が、配列の代わりに一つの文字列を使うことを認めます。

#### RFC7519-AUD-SINGLE-STRING 受信者が一つの `aud` の文字列表現

- **Source**：[RFC 7519 §4.1.3](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.3)
- **Quote**：`In the special case when the JWT has one audience, the "aud" value MAY be a single case-sensitive string containing a StringOrURI value.`
- **Level**：MAY
- **Actor**：Issuer
- **Condition**：JWT の受信者が一つである

この条項は、`aud` の値を配列とする RFC7519-AUD-FORMAT を緩めます。
原文は値の形として書き、主体を書いていません。`aud` の値を決めるのは Issuer なので、Actor を Issuer とします。

#### RFC7519-AUD-OPTIONAL `aud` の使用の任意性

- **Source**：[RFC 7519 §4.1.3](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.3)
- **Quote**：`Use of this claim is OPTIONAL.`
- **Level**：OPTIONAL
- **Actor**：Issuer

原文は使用の主体を書いていません。Claim を JWT に含めるかを決めるのは Issuer なので、Actor を Issuer とします。

### `exp`

#### RFC7519-EXP-REJECT `exp` 以後の受理の禁止

- **Source**：[RFC 7519 §4.1.4](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.4)
- **Quote**：`The "exp" (expiration time) claim identifies the expiration time on or after which the JWT MUST NOT be accepted for processing. The processing of the "exp" claim requires that the current date/time MUST be before the expiration date/time listed in the "exp" claim.`
- **Level**：MUST NOT
- **Actor**：Recipient

2 つの文は、現在時刻が `exp` の時刻より前の JWT だけを受理するという同じ規範を、受理しない側と受理する側から述べています。
原文は受動態で主体を書いていません。JWT を受け取って処理する主体を、§4.1.3 は「recipients」と呼ぶため、Actor を Recipient とします。
`exp` の使用は OPTIONAL なので（RFC7519-EXP-OPTIONAL）、この条項は `exp` を含まない JWT には適用されません。
RFC7519-EXP-LEEWAY が、この判定に猶予を認めます。

#### RFC7519-EXP-LEEWAY `exp` の判定の猶予

- **Source**：[RFC 7519 §4.1.4](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.4)
- **Quote**：`Implementers MAY provide for some small leeway, usually no more than a few minutes, to account for clock skew.`
- **Level**：MAY
- **Actor**：Implementer

この条項は RFC7519-EXP-REJECT を緩め、`exp` の時刻を過ぎた JWT を猶予の間だけ受理することを認めます。
「usually no more than a few minutes」は RFC 2119 の語を使っておらず、猶予の上限を定める規範ではありません。

#### RFC7519-EXP-NUMERICDATE `exp` の値の NumericDate への限定

- **Source**：[RFC 7519 §4.1.4](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.4)
- **Quote**：`Its value MUST be a number containing a NumericDate value.`
- **Level**：MUST
- **Actor**：Issuer

原文は値に課す要件として書き、主体を書いていません。`exp` の値を決めるのは Issuer なので、Actor を Issuer とします。
NumericDate の形式は RFC7519-NUMERICDATE-FORMAT が定めます。値が NumericDate でない JWT を受信者がどう扱うかを、原文は定めていません。

#### RFC7519-EXP-OPTIONAL `exp` の使用の任意性

- **Source**：[RFC 7519 §4.1.4](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.4)
- **Quote**：`Use of this claim is OPTIONAL.`
- **Level**：OPTIONAL
- **Actor**：Issuer

原文は使用の主体を書いていません。Claim を JWT に含めるかを決めるのは Issuer なので、Actor を Issuer とします。

### `nbf`

#### RFC7519-NBF-REJECT `nbf` より前の受理の禁止

- **Source**：[RFC 7519 §4.1.5](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.5)
- **Quote**：`The "nbf" (not before) claim identifies the time before which the JWT MUST NOT be accepted for processing. The processing of the "nbf" claim requires that the current date/time MUST be after or equal to the not-before date/time listed in the "nbf" claim.`
- **Level**：MUST NOT
- **Actor**：Recipient

2 つの文は、現在時刻が `nbf` の時刻以後の JWT だけを受理するという同じ規範を、受理しない側と受理する側から述べています。
原文は受動態で主体を書いていません。JWT を受け取って処理する主体を、§4.1.3 は「recipients」と呼ぶため、Actor を Recipient とします。
`nbf` の使用は OPTIONAL なので（RFC7519-NBF-OPTIONAL）、この条項は `nbf` を含まない JWT には適用されません。
RFC7519-NBF-LEEWAY が、この判定に猶予を認めます。

#### RFC7519-NBF-LEEWAY `nbf` の判定の猶予

- **Source**：[RFC 7519 §4.1.5](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.5)
- **Quote**：`Implementers MAY provide for some small leeway, usually no more than a few minutes, to account for clock skew.`
- **Level**：MAY
- **Actor**：Implementer

この条項は RFC7519-NBF-REJECT を緩め、`nbf` の時刻より前の JWT を猶予の間だけ受理することを認めます。
「usually no more than a few minutes」は RFC 2119 の語を使っておらず、猶予の上限を定める規範ではありません。

#### RFC7519-NBF-NUMERICDATE `nbf` の値の NumericDate への限定

- **Source**：[RFC 7519 §4.1.5](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.5)
- **Quote**：`Its value MUST be a number containing a NumericDate value.`
- **Level**：MUST
- **Actor**：Issuer

原文は値に課す要件として書き、主体を書いていません。`nbf` の値を決めるのは Issuer なので、Actor を Issuer とします。
NumericDate の形式は RFC7519-NUMERICDATE-FORMAT が定めます。値が NumericDate でない JWT を受信者がどう扱うかを、原文は定めていません。

#### RFC7519-NBF-OPTIONAL `nbf` の使用の任意性

- **Source**：[RFC 7519 §4.1.5](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.5)
- **Quote**：`Use of this claim is OPTIONAL.`
- **Level**：OPTIONAL
- **Actor**：Issuer

原文は使用の主体を書いていません。Claim を JWT に含めるかを決めるのは Issuer なので、Actor を Issuer とします。

### `iat`

#### RFC7519-IAT-NUMERICDATE `iat` の値の NumericDate への限定

- **Source**：[RFC 7519 §4.1.6](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.6)
- **Quote**：`Its value MUST be a number containing a NumericDate value.`
- **Level**：MUST
- **Actor**：Issuer

原文は値に課す要件として書き、主体を書いていません。`iat` の値を決めるのは Issuer なので、Actor を Issuer とします。
NumericDate の形式は RFC7519-NUMERICDATE-FORMAT が定めます。値が NumericDate でない JWT を受信者がどう扱うかを、原文は定めていません。
`iat` は JWT の経過時間の判定に使えます（原文）。ただし RFC 7519 は、`iat` による受理の判定も、その猶予も定めていません。

#### RFC7519-IAT-OPTIONAL `iat` の使用の任意性

- **Source**：[RFC 7519 §4.1.6](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.6)
- **Quote**：`Use of this claim is OPTIONAL.`
- **Level**：OPTIONAL
- **Actor**：Issuer

原文は使用の主体を書いていません。Claim を JWT に含めるかを決めるのは Issuer なので、Actor を Issuer とします。

### `jti`

#### RFC7519-JTI-UNIQUE `jti` の値の衝突の防止

- **Source**：[RFC 7519 §4.1.7](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.7)
- **Quote**：`The identifier value MUST be assigned in a manner that ensures that there is a negligible probability that the same value will be accidentally assigned to a different data object; if the application uses multiple issuers, collisions MUST be prevented among values produced by different issuers as well.`
- **Level**：MUST
- **Actor**：Issuer

原文は受動態で主体を書いていません。`jti` の値を割り当てるのは Issuer なので、Actor を Issuer とします。
複数の Issuer の間の衝突は、RFC7519-JTI-ISSUERS-UNIQUE が扱います。

#### RFC7519-JTI-ISSUERS-UNIQUE 複数の Issuer の間の `jti` の衝突の防止

- **Source**：[RFC 7519 §4.1.7](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.7)
- **Quote**：`The identifier value MUST be assigned in a manner that ensures that there is a negligible probability that the same value will be accidentally assigned to a different data object; if the application uses multiple issuers, collisions MUST be prevented among values produced by different issuers as well.`
- **Level**：MUST
- **Actor**：Application
- **Condition**：アプリケーションが複数の Issuer を使う

原文は受動態で主体を書いていません。条件の主語が複数の Issuer を使うアプリケーションであり、Issuer どうしの間の衝突は一つの Issuer の中では防げないため、Actor を Application とします。

#### RFC7519-JTI-FORMAT `jti` の値の形式

- **Source**：[RFC 7519 §4.1.7](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.7)
- **Quote**：`The "jti" value is a case-sensitive string.`
- **Level**：Definition

`jti` は JWT のリプレイの防止に使えます（原文）。

#### RFC7519-JTI-OPTIONAL `jti` の使用の任意性

- **Source**：[RFC 7519 §4.1.7](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.7)
- **Quote**：`Use of this claim is OPTIONAL.`
- **Level**：OPTIONAL
- **Actor**：Issuer

原文は使用の主体を書いていません。Claim を JWT に含めるかを決めるのは Issuer なので、Actor を Issuer とします。

### JWT String Comparison

#### RFC7519-STRING-COMPARE JSON の文字列比較の規則

- **Source**：[RFC 7519 §7.3](https://www.rfc-editor.org/rfc/rfc7519.html#section-7.3)
- **Quote**：`These comparison rules MUST be used for all JSON string comparisons except in cases where the definition of the member explicitly calls out that a different comparison rule is to be used for that member value.`
- **Level**：MUST
- **Actor**：Implementation
- **Condition**：メンバーの定義が、そのメンバーの値に別の比較規則を使うことを明示していない

「These comparison rules」は、RFC 7159 §8.3 の比較規則（RFC7159-STRING-COMPARE）を指します。§7.3 は、比較が等しいかどうかの判定だけなので、メンバー名とメンバーの値の両方に同じ規則を使えるとしています。
したがって Claim 名の `aud` と `AUD` は異なる名前です。
RFC 7519 の中で別の比較規則を使うのは、`typ` と `cty` の値だけです。
原文は受動態で主体を書いていません。§7.3 は JWT の処理に伴う比較を扱い、RFC 7519 §4 は JWT を処理する主体を「implementations」と呼ぶため、Actor を Implementation とします。
大文字と小文字を区別しない情報（DNS 名など）を `iss` のような大文字と小文字を区別する値に含め、複数の当事者が同じ値を生成して比較する場合、アプリケーションはその部分の正規の表記の取り決め（小文字にそろえるなど）を定める必要があり得ます（原文は小文字の「may need to」）。生成者が出した値をそのまま使い、独立に生成した値と比較しないなら、表記は問題になりません。

## 3. RFC 8725 JWT Best Current Practices

RFC 8725 §1 は、この文書の推奨を最低限のものとし、これを参照するほかの仕様がより厳しい要件を定め得るとしています。

### Algorithms

#### RFC8725-ALG-SET-SPECIFY 対応するアルゴリズムの集合の指定

- **Source**：[RFC 8725 §3.1](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.1)
- **Quote**：`Libraries MUST enable the caller to specify a supported set of algorithms and MUST NOT use any other algorithms when performing cryptographic operations.`
- **Level**：MUST
- **Actor**：Library

#### RFC8725-ALG-SET-RESTRICT 指定の外のアルゴリズムの不使用

- **Source**：[RFC 8725 §3.1](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.1)
- **Quote**：`Libraries MUST enable the caller to specify a supported set of algorithms and MUST NOT use any other algorithms when performing cryptographic operations.`
- **Level**：MUST NOT
- **Actor**：Library
- **Condition**：暗号操作を行う

「any other algorithms」は、呼び出し元が RFC8725-ALG-SET-SPECIFY によって指定した集合の外のアルゴリズムです。

#### RFC8725-ALG-HEADER-MATCH `alg` と暗号操作のアルゴリズムの一致

- **Source**：[RFC 8725 §3.1](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.1)
- **Quote**：`The library MUST ensure that the "alg" or "enc" header specifies the same algorithm that is used for the cryptographic operation.`
- **Level**：MUST
- **Actor**：Library

`enc` は JWE の Header Parameter です。

#### RFC8725-KEY-ALG-SINGLE 一つの鍵を一つのアルゴリズムだけで使うこと

- **Source**：[RFC 8725 §3.1](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.1)
- **Quote**：`Moreover, each key MUST be used with exactly one algorithm, and this MUST be checked when the cryptographic operation is performed.`
- **Level**：MUST
- **Actor**：Application

原文は受動態で主体を書いていません。暗号操作に使う鍵とアルゴリズムを選んで Library に与えるのは Application であり（§3.3 の「the keys and algorithms supplied by the application」）、Library の確認だけでは、Application が一つの鍵を複数のアルゴリズムで使うことを防げないため、Actor を Application とします。
暗号操作の時点の確認は、主体が異なるため RFC8725-KEY-ALG-CHECK に分けます。

#### RFC8725-KEY-ALG-CHECK 暗号操作の時点の鍵とアルゴリズムの対応の確認

- **Source**：[RFC 8725 §3.1](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.1)
- **Quote**：`Moreover, each key MUST be used with exactly one algorithm, and this MUST be checked when the cryptographic operation is performed.`
- **Level**：MUST
- **Actor**：Library
- **Condition**：暗号操作を行う

原文は受動態で主体を書いていません。§3.1 は暗号操作を行う主体を Library としているため、Actor を Library とします。
確かめる内容は、RFC8725-KEY-ALG-SINGLE の、鍵が一つのアルゴリズムだけで使われることです。

#### RFC8725-ALG-CURRENT 現行のアルゴリズムだけの許可

- **Source**：[RFC 8725 §3.2](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.2)
- **Quote**：`Therefore, applications MUST only allow the use of cryptographically current algorithms that meet the security requirements of the application.`
- **Level**：MUST
- **Actor**：Application

「Therefore」の前で、§3.2 は、どのアルゴリズムを使えるかはアプリケーションが決めることだとする RFC 7515 §5.2 の文を引用しています。

#### RFC8725-ALG-AGILITY アルゴリズムを入れ替えられる設計

- **Source**：[RFC 8725 §3.2](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.2)
- **Quote**：`Applications MUST therefore be designed to enable cryptographic agility.`
- **Level**：MUST
- **Actor**：Application

現行のアルゴリズムの集合は、新しいアルゴリズムの導入と、弱点が見つかったアルゴリズムの廃止によって変わります（原文の直前の文）。

#### RFC8725-ALG-NONE-GENERATE `none` による JWT の生成

- **Source**：[RFC 8725 §3.2](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.2)
- **Quote**：`JWT libraries SHOULD NOT generate JWTs using "none" unless explicitly requested to do so by the caller.`
- **Level**：SHOULD NOT
- **Actor**：Library
- **Condition**：呼び出し元が明示的に要求していない

§3.2 は、TLS のような転送路で端から端まで暗号的に保護された JWT では、`none` の使用が許容され得るとしています。

#### RFC8725-ALG-NONE-CONSUME `none` を使う JWT の受け入れ

- **Source**：[RFC 8725 §3.2](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.2)
- **Quote**：`Similarly, JWT libraries SHOULD NOT consume JWTs using "none" unless explicitly requested by the caller.`
- **Level**：SHOULD NOT
- **Actor**：Library
- **Condition**：呼び出し元が明示的に要求していない

### Cryptographic Operations

#### RFC8725-CRYPTO-VALIDATE すべての暗号操作の検証

- **Source**：[RFC 8725 §3.3](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.3)
- **Quote**：`All cryptographic operations used in the JWT MUST be validated and the entire JWT MUST be rejected if any of them fail to validate.`
- **Level**：MUST
- **Actor**：Library

原文は受動態で主体を書いていません。§3.3 の次の文が検証に使う鍵とアルゴリズムを「supplied by the application」とすることから、検証する主体はアプリケーションとは別の主体だと読めます。また、§3.1 は暗号操作を行う主体を Library とし、§3.3 が対策する §2.3 の脅威は「Some libraries」が内側の署名を検証しないことです。そのため Actor を Library とします。
ただし §1.1 は、Library が備えない仕組みを、Library を使うコードの実装者も対象としています。使う Library がこの検証を行わないときは、Library を使うコードが検証を補うことになります。
検証と、検証に失敗したときの拒否は、「すべての暗号操作の検証に成功した JWT だけを受理する」という一つの判定を表すため、一つの条項にまとめます。

#### RFC8725-CRYPTO-NESTED-VALIDATE Nested JWT の内側と外側の暗号操作の検証

- **Source**：[RFC 8725 §3.3](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.3)
- **Quote**：`This is true not only of JWTs with a single set of Header Parameters but also for Nested JWTs in which both outer and inner operations MUST be validated using the keys and algorithms supplied by the application.`
- **Level**：MUST
- **Actor**：Library
- **Condition**：JWT が Nested JWT である

原文は受動態で主体を書いていません。Actor を Library とする根拠は RFC8725-CRYPTO-VALIDATE と同じです。
内側と外側の暗号操作は、どちらもアプリケーションが与えた鍵とアルゴリズムで検証します。

### UTF-8

#### RFC8725-UTF8-IMPLEMENTATION 実装による UTF-8 だけの使用

- **Source**：[RFC 8725 §3.7](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.7)
- **Quote**：`[RFC7515], [RFC7516], and [RFC7519] all specify that UTF-8 be used for encoding and decoding JSON used in Header Parameters and JWT Claims Sets. This is also in line with the latest JSON specification [RFC8259]. Implementations and applications MUST do this and not use or admit the use of other Unicode encodings for these purposes.`
- **Level**：MUST
- **Actor**：Implementation

「do this」は、Header Parameter と JWT Claims Set の JSON の符号化と復号に UTF-8 を使うことを指します。
原文は Implementation と Application の両方を主語にしているため、主体ごとに RFC8725-UTF8-APPLICATION と条項を分けます。

#### RFC8725-UTF8-APPLICATION アプリケーションによる UTF-8 だけの使用

- **Source**：[RFC 8725 §3.7](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.7)
- **Quote**：`[RFC7515], [RFC7516], and [RFC7519] all specify that UTF-8 be used for encoding and decoding JSON used in Header Parameters and JWT Claims Sets. This is also in line with the latest JSON specification [RFC8259]. Implementations and applications MUST do this and not use or admit the use of other Unicode encodings for these purposes.`
- **Level**：MUST
- **Actor**：Application

「do this」は、Header Parameter と JWT Claims Set の JSON の符号化と復号に UTF-8 を使うことを指します。
原文は Implementation と Application の両方を主語にしているため、主体ごとに RFC8725-UTF8-IMPLEMENTATION と条項を分けます。

### Issuer and Subject

#### RFC8725-ISS-KEY-VALIDATE 鍵が Issuer に属することの検証

- **Source**：[RFC 8725 §3.8](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.8)
- **Quote**：`When a JWT contains an "iss" (issuer) claim, the application MUST validate that the cryptographic keys used for the cryptographic operations in the JWT belong to the issuer. If they do not, the application MUST reject the JWT.`
- **Level**：MUST
- **Actor**：Application
- **Condition**：JWT が `iss` を含む

検証と、鍵が Issuer に属さないときの拒否は、「暗号操作に使った鍵が `iss` の Issuer に属する JWT だけを受理する」という一つの判定を表すため、一つの条項にまとめます。
鍵が Issuer に属することを決める方法は、アプリケーション固有です。§3.8 は例として、OpenID Connect の Issuer の `https` の URL が参照するメタデータの `jwks_uri` から、Issuer の鍵を JWK Set として取得する方法を挙げています。

#### RFC8725-SUB-VALIDATE `sub` の検証

- **Source**：[RFC 8725 §3.8](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.8)
- **Quote**：`Similarly, when the JWT contains a "sub" (subject) claim, the application MUST validate that the subject value corresponds to a valid subject and/or issuer-subject pair at the application. This may include confirming that the issuer is trusted by the application. If the issuer, subject, or the pair are invalid, the application MUST reject the JWT.`
- **Level**：MUST
- **Actor**：Application
- **Condition**：JWT が `sub` を含む

検証と、無効なときの拒否は、「Issuer、Subject、その組がアプリケーションにとって有効な JWT だけを受理する」という一つの判定を表すため、一つの条項にまとめます。
何を有効な Subject とするかは、アプリケーションが決めます。Issuer を信頼するかの確認を含め得るという 2 文目は、小文字の「may」です。

### Audience

#### RFC8725-AUD-REQUIRED 複数の受信者に発行する Issuer の `aud`

- **Source**：[RFC 8725 §3.9](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.9)
- **Quote**：`If the same issuer can issue JWTs that are intended for use by more than one relying party or application, the JWT MUST contain an "aud" (audience) claim that can be used to determine whether the JWT is being used by an intended party or was substituted by an attacker at an unintended party.`
- **Level**：MUST
- **Actor**：Issuer
- **Condition**：同じ Issuer が、複数の Relying Party または Application による使用を意図した JWT を発行し得る

原文は JWT に課す要件として書き、主体を書いていません。JWT に `aud` を含めるのは Issuer なので、Actor を Issuer とします。
`aud` は、JWT が意図された相手に使われているのか、意図されない相手のもとで攻撃者に差し替えられたのかを判定できる値でなければなりません。すべての受信者に共通の値を入れても、この条項は満たされません。

#### RFC8725-AUD-VALIDATE 複数の受信者に発行する Issuer の JWT の `aud` の検証

- **Source**：[RFC 8725 §3.9](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.9)
- **Quote**：`In such cases, the relying party or application MUST validate the audience value, and if the audience value is not present or not associated with the recipient, it MUST reject the JWT.`
- **Level**：MUST
- **Actor**：Application
- **Condition**：同じ Issuer が、複数の Relying Party または Application による使用を意図した JWT を発行し得る

原文は主体を「the relying party or application」と書いています。relying party と application は、どちらも JWT を受け取る側（同じ文の「the recipient」）の呼び方で、受け取る側がどちらであっても義務を負います。RFC 8725 は §3.2、§3.8、§3.10 で JWT を受け取って使う主体を「application」と呼ぶため、Actor を Application とします。
検証と、`aud` がないときや受信者に結び付かないときの拒否は、「受信者に結び付く `aud` を含む JWT だけを受理する」という一つの判定を表すため、一つの条項にまとめます。
RFC 7519 の RFC7519-AUD-REJECT が `aud` が存在する JWT だけに適用されるのに対し、この条項は、条件を満たすとき `aud` がない JWT も拒否させます。

### Received Header Parameters

RFC 8725 §3.10 は、`kid` の値を検証または無害化して、鍵の検索が SQL や LDAP のインジェクションを招かないようにすることを、小文字の「should」で述べています。

#### RFC8725-JKU-X5U-SSRF `jku` と `x5u` の URL による SSRF の防止

- **Source**：[RFC 8725 §3.10](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.10)
- **Quote**：`Similarly, blindly following a "jku" (JWK set URL) or "x5u" (X.509 URL) header, which may contain an arbitrary URL, could result in server-side request forgery (SSRF) attacks. Applications SHOULD protect against such attacks, e.g., by matching the URL to a whitelist of allowed locations and ensuring no cookies are sent in the GET request.`
- **Level**：SHOULD
- **Actor**：Application

許可した場所の一覧と URL を照合すること、GET の要求で Cookie を送らないことは、防ぐ手段の例であり、手段を限定する規範ではありません。

### Explicit Typing and Kinds of JWT

#### RFC8725-KIND-CONFUSION-MITIGATE JWT の種類の取り違えによる差し替えへの対策

- **Source**：[RFC 8725 §2.8](https://www.rfc-editor.org/rfc/rfc8725.html#section-2.8)
- **Quote**：`If the JWT could be used in an application context in which it could be confused with other kinds of JWTs, then mitigations MUST be employed to prevent these substitution attacks.`
- **Level**：MUST
- **Actor**：Application
- **Condition**：JWT が、ほかの種類の JWT と取り違えられ得るアプリケーションの文脈で使われ得る

原文は受動態で主体を書いていません。§3.12 は、差し替えを防ぐ手段を採る主体を「application developers」と書いているため、Actor を Application とします。
§2.8 は対策として §3.8、§3.9、§3.11、§3.12 を挙げており、RFC8725-ISS-KEY-VALIDATE、RFC8725-SUB-VALIDATE、RFC8725-AUD-REQUIRED、RFC8725-AUD-VALIDATE、RFC8725-TYP-PREFIX-OMIT、RFC8725-TYP-MEDIA-TYPE、RFC8725-TYP-NESTED、RFC8725-TYP-NEW-USE、RFC8725-KIND-EXCLUSIVE がそれに当たります。この条項は、対策の手段を限定しません。

#### RFC8725-TYP-PREFIX-OMIT `typ` の値の `application/` の省略

- **Source**：[RFC 8725 §3.11](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.11)
- **Quote**：`Per the definition of "typ" in Section 4.1.9 of [RFC7515], it is RECOMMENDED that the "application/" prefix be omitted from the "typ" value. Therefore, for example, the "typ" value used to explicitly include a type for a SET SHOULD be "secevent+jwt".`
- **Level**：RECOMMENDED
- **Actor**：Issuer

2 つ目の文は、1 つ目の文の推奨を Security Event Token（SET）に当てはめた例です。
原文は値に課す要件として書き、主体を書いていません。`typ` の値を JWT に入れるのは Issuer なので、Actor を Issuer とします。

#### RFC8725-TYP-MEDIA-TYPE 明示的な型付けのメディアタイプ名の形式

- **Source**：[RFC 8725 §3.11](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.11)
- **Quote**：`When explicit typing is employed for a JWT, it is RECOMMENDED that a media type name of the format "application/example+jwt" be used, where "example" is replaced by the identifier for the specific kind of JWT.`
- **Level**：RECOMMENDED
- **Actor**：Application
- **Condition**：JWT に明示的な型付けを使う

原文は受動態で主体を書いていません。型の選択は JWT の種類のプロファイルを定める側が行い、§3.12 は種類の取り違えを防ぐ手段（明示的な型付けを含む）を採る主体を「application developers」と書いているため、Actor を Application とします。
`typ` の値には、RFC8725-TYP-PREFIX-OMIT により、`application/` を除いた `example+jwt` を書きます。

#### RFC8725-TYP-NESTED Nested JWT の内側の `typ`

- **Source**：[RFC 8725 §3.11](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.11)
- **Quote**：`When applying explicit typing to a Nested JWT, the "typ" Header Parameter containing the explicit type value MUST be present in the inner JWT of the Nested JWT (the JWT whose payload is the JWT Claims Set).`
- **Level**：MUST
- **Actor**：Issuer
- **Condition**：Nested JWT に明示的な型付けを使う

原文は受動態で主体を書いていません。JOSE Header に `typ` を入れるのは Issuer なので、Actor を Issuer とします。

#### RFC8725-TYP-NEW-USE 新しい用途での明示的な型付け

- **Source**：[RFC 8725 §3.11](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.11)
- **Quote**：`Explicit typing is RECOMMENDED for new uses of JWTs.`
- **Level**：RECOMMENDED
- **Actor**：Application
- **Condition**：JWT の新しい用途である

原文は受動態で主体を書いていません。Actor を Application とする根拠は RFC8725-TYP-MEDIA-TYPE と同じです。
§3.12 の最後の文は、同じ推奨を繰り返しています。
既存の種類の JWT の検証規則は `typ` を確かめないことが多いため、明示的な型付けだけでは既存の種類と区別できない場合があります（原文）。

#### RFC8725-KIND-EXCLUSIVE 種類の異なる JWT の検証規則の排他性

- **Source**：[RFC 8725 §3.12](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.12)
- **Quote**：`If more than one kind of JWT can be issued by the same issuer, the validation rules for those JWTs MUST be written such that they are mutually exclusive, rejecting JWTs of the wrong kind.`
- **Level**：MUST
- **Actor**：Application
- **Condition**：同じ Issuer が複数の種類の JWT を発行し得る

原文は受動態で主体を書いていません。§3.12 は、検証規則を排他にする手段を採る主体を「application developers」と書いているため、Actor を Application とします。
§3.12 は、排他にする手段として、種類ごとに異なる `typ`、必須の Claim やその値、必須の Header Parameter やその値、鍵、`aud`、Issuer を挙げ、最適な組み合わせはアプリケーション固有であるとしています。

## 4. References

- [RFC 2119 — Key words for use in RFCs to Indicate Requirement Levels](https://www.rfc-editor.org/rfc/rfc2119.html)（1997）
- [RFC 7159 — The JavaScript Object Notation (JSON) Data Interchange Format](https://www.rfc-editor.org/rfc/rfc7159.html)（2014、§8.3）
- [RFC 7519 — JSON Web Token (JWT)](https://www.rfc-editor.org/rfc/rfc7519.html)（2015、§2、§4、§4.1、§7.3）
- [RFC 8725 — JSON Web Token Best Current Practices](https://www.rfc-editor.org/rfc/rfc8725.html)（2020、§1、§2.8、§3。RFC 7519 の更新）
