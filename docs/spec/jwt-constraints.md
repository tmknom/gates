# JWT Constraints

## 1. Scope and Evidence

この文書は、JWT / JWS / JWK / OpenID Connect の標準仕様から、署名・鍵・Claims・Discovery の検証に関係する規定を整理したものです。標準仕様の代替ではなく、規定の適用対象と条件はリンク先の原文を優先します。

- **対象**：JWS Compact Serialization、RSA 署名、JWT Claims、JWK Set、OpenID Provider Metadata と ID Token の検証に関係する規定
- **対象外**：OAuth 2.0 の認可フロー全体、JWE による暗号化、各署名方式の網羅的な解説

RFC の **MUST / SHOULD / MAY** は原文の規範的強度を表します。OpenID Connect Core / Discovery の規定は、JWT 一般の仕様とは適用対象が異なります。特に ID Token や OpenID Connect の認証フローに限定される条件は、この文書の §5 で区別します。

## 2. JWS and JOSE Header

### 2.1 構造、署名、アルゴリズム

**標準仕様**：JWS Compact Serialization は、Base64URL エンコードされた Protected Header、Payload、Signature の3部分を `.` で連結します。
JOSE Header の `alg` は、存在することと、理解・処理されることが **MUST** です。
`alg` が対応しないアルゴリズムを表す場合や、署名者に結び付くそのアルゴリズム用の鍵がない場合、署名は無効です。
署名は、`alg` が正確に表す方式で検証します（[RFC 7515 §3.1、§4.1.1、§5.2](https://www.rfc-editor.org/rfc/rfc7515.html#section-5.2)）。

**標準仕様**：`RS256` は RSASSA-PKCS1-v1_5 と SHA-256 の組み合わせであり、この方式で使う RSA 鍵は2048 bit 以上を **MUST** とします。`PS256` は別の署名方式です（[RFC 7518 §3.3、§3.5](https://www.rfc-editor.org/rfc/rfc7518.html#section-3.3)）。

**標準仕様**：JWT ライブラリは、対応するアルゴリズムの集合を呼び出し元が指定できるようにし、それ以外を暗号操作に使ってはいけません（**MUST NOT**）。
ライブラリは `alg` が実際の暗号操作と同じ方式を示すことを確かめます（**MUST**）。
各鍵は一つのアルゴリズムだけで使い、そのことを暗号操作時に確かめます（**MUST**）。
アプリケーションは、セキュリティ要件を満たす現行のアルゴリズムだけを許可します（**MUST**）（[RFC 8725 §3.1–§3.2](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.1)）。
署名を検証できても、アルゴリズムをアプリケーションが受け入れないなら、JWS を無効とみなすことが **SHOULD** です（[RFC 7515 §5.2](https://www.rfc-editor.org/rfc/rfc7515.html#section-5.2)）。

### 2.2 鍵の識別と Header の拡張

| JOSE Header | 標準仕様上の位置づけ | 出典 |
|---|---|---|
| `alg` | 署名方式。**必須**。値を検証時に処理する | [RFC 7515 §4.1.1](https://www.rfc-editor.org/rfc/rfc7515.html#section-4.1.1) |
| `kid` | 鍵の選択に使う識別用の手掛かり。**任意**。文字列は大文字小文字を区別する | [RFC 7515 §4.1.4](https://www.rfc-editor.org/rfc/rfc7515.html#section-4.1.4) |
| `typ` | JWS 全体のメディアタイプを宣言する。**任意**。JWS の実装は無視し、処理はアプリケーションが行う。JWT の種類の取り違えを防ぐ一手段 | [RFC 7515 §4.1.9](https://www.rfc-editor.org/rfc/rfc7515.html#section-4.1.9)、[RFC 8725 §3.11–§3.12](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.11) |
| `crit` | 列挙された拡張 Header は理解・処理する必要がある。未対応の拡張があれば JWS は無効。項目自体は **任意** だが、実装は `crit` を理解・処理することが **MUST** | [RFC 7515 §4.1.11](https://www.rfc-editor.org/rfc/rfc7515.html#section-4.1.11) |
| `jku`, `jwk`, `x5u`, `x5c`, `x5t`, `x5t#S256` | 公開鍵・証明書に関係する情報を運ぶ **任意** の Header | [RFC 7515 §4.1.2–§4.1.8](https://www.rfc-editor.org/rfc/rfc7515.html#section-4.1.2) |

**標準仕様**：JOSE Header の名前は一意であることが **MUST** です。パーサーは重複する名前を拒否するか、最後に現れた同名フィールドのみを採用します（**MUST**）。
RFC 7515 が理解を必須と指定した Header、使用するアルゴリズムが要求する Header、`crit` に列挙された Header を除き、未理解の Header は無視することが **MUST** です。
したがって、これらに当たらない未理解の Header を理由に JWS を拒否することは、この規定に反します（[RFC 7515 §4、§4.1.11](https://www.rfc-editor.org/rfc/rfc7515.html#section-4.1.11)）。

**標準仕様**：署名者は、鍵を特定できる情報を Header に含めることが **SHOULD** です。ただし、アプリケーションが別の手段や取り決めで鍵を決める場合は除きます。
鍵を必要とするアルゴリズム（`none` 以外のすべて）で鍵を特定できなければ、署名の検証は失敗します（[RFC 7515 §6](https://www.rfc-editor.org/rfc/rfc7515.html#section-6)）。

**標準仕様（OIDC 固有）**：OpenID Connect Core §10.1 は、RSA または ECDSA の署名者に、公開した JWK Set の公開鍵に対応する秘密鍵を使うことを求めます。
参照する JWK Set に複数の鍵があれば、JOSE Header に `kid` を指定することが **MUST** で、鍵の用途は署名に対応していることが **MUST** です。
したがって、RFC 7515 の `kid` が任意という規定だけで、OIDC のすべてのケースにおける省略を許容できるとはいえません（[OpenID Connect Core 1.0 §10.1](https://openid.net/specs/openid-connect-core-1_0.html#Signing)）。

**標準仕様**：発行側は、`crit` に空配列、重複、JOSE Header に存在しない名前、JWS / JWA が定義済みの名前を含めてはいけません（**MUST NOT**）。`crit` は JWS Protected Header に置き、完全性保護を受けることが **MUST** です。受信者は、これらの条件への違反を理由に JWS を無効とみなすことができます（**MAY**）。未対応の拡張を含む場合の無効化とは規範の強さが異なります（[RFC 7515 §4.1.11](https://www.rfc-editor.org/rfc/rfc7515.html#section-4.1.11)）。

**標準仕様**：JWT に JWS の非エンコード Payload オプション（`b64:false`）を使用することは **MUST NOT** です。
`b64` を使う場合は、`crit` に `b64` を列挙することが **MUST** です（[RFC 7797 §6–§7](https://www.rfc-editor.org/rfc/rfc7797.html#section-7)）。

## 3. JWK and JWK Set

### 3.1 鍵の属性

| JWK Member | 標準仕様上の位置づけ | 出典 |
|---|---|---|
| `kty` | 鍵の種類。**必須**。RSA 公開鍵なら `RSA` | [RFC 7517 §4.1](https://www.rfc-editor.org/rfc/rfc7517.html#section-4.1)、[RFC 7518 §6.3](https://www.rfc-editor.org/rfc/rfc7518.html#section-6.3) |
| `n`, `e` | RSA 公開鍵の法と公開指数。**RSA 公開鍵では必須**。Base64urlUInt で表現 | [RFC 7518 §6.3.1](https://www.rfc-editor.org/rfc/rfc7518.html#section-6.3.1) |
| `use` | 鍵の用途（署名 `sig`、暗号化 `enc`）。アプリケーションが求めない限り **任意** | [RFC 7517 §4.2](https://www.rfc-editor.org/rfc/rfc7517.html#section-4.2) |
| `key_ops` | 鍵操作の集合。検証は `verify`。アプリケーションが求めない限り **任意** | [RFC 7517 §4.3](https://www.rfc-editor.org/rfc/rfc7517.html#section-4.3) |
| `alg` | 鍵を使用する想定アルゴリズム。**任意**。JOSE Header の `alg` と役割が違う | [RFC 7517 §4.4](https://www.rfc-editor.org/rfc/rfc7517.html#section-4.4) |
| `kid` | JWK Set 内などでの鍵選択に用いる識別子。**任意**。異なる鍵では異なる値を使うことが **SHOULD** | [RFC 7517 §4.5](https://www.rfc-editor.org/rfc/rfc7517.html#section-4.5) |

**標準仕様**：`use` と `key_ops` の併用は **SHOULD NOT** ですが、両方ある場合は意味が整合していることが **MUST** です。`key_ops` の配列内で操作値を重複させることは **MUST NOT** です（[RFC 7517 §4.3](https://www.rfc-editor.org/rfc/rfc7517.html#section-4.3)）。

**標準仕様**：JWK Set は `keys` 配列を持つ JSON オブジェクトです。
実装は、`kty` を理解できない JWK、必須のメンバーが欠けた JWK、値が対応範囲外の JWK を無視することが **SHOULD** です。
異なる鍵でも、`kty` が異なり、アプリケーションが同等の代替とみなす鍵のように、`kid` が同じ値になる例が示されています（[RFC 7517 §5、§4.5](https://www.rfc-editor.org/rfc/rfc7517.html#section-5)）。

### 3.2 OpenID Connect における鍵用途

**標準仕様（OIDC 固有）**：OpenID Connect Discovery は、同じ JWK Set に署名用と暗号化用の鍵の両方を公開するとき、**すべての鍵** に `use` を要求します。したがって RFC 7517 の「任意」だけから、OIDC の JWK Set でも常に欠落を許容できると結論づけられません。
JWK Set に秘密鍵と対称鍵の値を含めることは **MUST NOT** です（[OpenID Connect Discovery 1.0 §3 `jwks_uri`](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)）。

## 4. JWT Claims

| Claim | 標準仕様 | 出典 |
|---|---|---|
| `iss` | 発行者。JWT 一般では **任意**。値は大文字小文字を区別する文字列。処理は一般にアプリケーション固有 | [RFC 7519 §4.1.1](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.1) |
| `sub` | 主体。JWT 一般では **任意**。値は Issuer 内で局所的に一意、または全体として一意であることが **MUST**。処理は一般にアプリケーション固有。OIDC Core は Subject Identifier の再割当てを禁止 | [RFC 7519 §4.1.2](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.2)、[OIDC Core §8](https://openid.net/specs/openid-connect-core-1_0.html#SubjectIDTypes) |
| `aud` | 受信対象。JWT 一般では **任意**。文字列または文字列の配列を使用できる。存在する場合、受信者自身が `aud` に含まれなければ拒否することが **MUST**。値の解釈は一般にアプリケーション固有 | [RFC 7519 §4.1.3](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.3) |
| `exp` | 失効時刻。JWT 一般では **任意**。受理する時刻は `exp` より前であること | [RFC 7519 §4.1.4](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.4) |
| `nbf` | 受理開始時刻。JWT 一般では **任意**。受理する時刻は `nbf` 以降であること | [RFC 7519 §4.1.5](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.5) |
| `iat` | 発行時刻。JWT 一般では **任意** | [RFC 7519 §4.1.6](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.6) |

**標準仕様**：`exp`、`nbf`、`iat` の値は JSON の**数値**で表す NumericDate です。整数に限定する規定ではありませんが、JSON の数字文字列はこの形式ではありません（[RFC 7519 §2、§4.1.4–§4.1.6](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.4)）。
時計差を考慮して数分程度の猶予を設けてよい（**MAY**）のは `exp` と `nbf` の処理です。
`iat` は発行時刻を表し、JWT の経過時間の判定に使えます。
`iat` について、RFC 7519 は受理の判定も猶予も定めていません（[RFC 7519 §4.1.6](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.6)）。

**標準仕様**：JWT Claims Set の Claim 名は一意であることが **MUST** です。パーサーは重複を拒否するか、最後の同名フィールドのみを採用します（**MUST**）。
アプリケーション固有の必須 Claim は JWT 一般の仕様だけでは決まりません。
そうした要求がなければ、理解できない Claim は無視することが **MUST** です（[RFC 7519 §4](https://www.rfc-editor.org/rfc/rfc7519.html#section-4)）。

**標準仕様**：Claim 名を含む JSON 文字列の比較には RFC 7519 §7.3 の比較規則を用いることが **MUST** です。
この規則は RFC 7159 §8.3 に基づき、エスケープを解いた文字列を Unicode のコードユニット単位で比較します。
したがって `aud` と `AUD` は異なる Claim 名です。
`typ` と `cty` の値には例外があります（[RFC 7519 §7.3](https://www.rfc-editor.org/rfc/rfc7519.html#section-7.3)、[RFC 7159 §8.3](https://www.rfc-editor.org/rfc/rfc7159.html#section-8.3)）。

**標準仕様（OIDC 固有）**：OpenID Connect のメッセージの文字列比較では、Unicode 正規化が **MUST NOT** で、コードポイント単位の等価比較が **MUST** です（[OpenID Connect Core 1.0 §14](https://openid.net/specs/openid-connect-core-1_0.html#StringOps)、[OpenID Connect Discovery 1.0 §5](https://openid.net/specs/openid-connect-discovery-1_0.html#StringOps)）。

## 5. OpenID Connect and Discovery

**標準仕様（OIDC ID Token）**：OpenID Connect Core 1.0 の ID Token は `iss`、`sub`、`aud`、`exp`、`iat` などを必須 Claim とします。OpenID Connect Core は `nbf` を定めていません。Authorization Code Flow で受信する ID Token については、Issuer、Audience、署名などを検証します。
特に、`aud` に登録済みの `client_id` が含まれることを検証します（**MUST**）。
`aud` は複数の値を持てますが、Client が信頼しない Audience を追加で含む ID Token は拒否することが **MUST** です。
Token Endpoint との直接通信で受け取る ID Token（Authorization Code Flow はこれに当たる）は、署名の確認に代えて TLS サーバー検証で Issuer を検証できます（**MAY**）。
それ以外の ID Token は、署名の検証が **MUST** です（[OpenID Connect Core 1.0 §2、§3.1.3.7](https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation)）。
Implicit Flow の検証は、§3.1.3.7 の規則に差分を加えて定められています（[OpenID Connect Core 1.0 §3.2.2.11](https://openid.net/specs/openid-connect-core-1_0.html#ImplicitIDTValidation)）。
Hybrid Flow では、Authorization Endpoint から返る ID Token は Implicit Flow と同じ方法で、Token Endpoint から返る ID Token は §3.1.3.7 と同じ方法で検証します（**MUST**）（[OpenID Connect Core 1.0 §3.3.2.12](https://openid.net/specs/openid-connect-core-1_0.html#HybridIDTValidation)、[§3.3.3.7](https://openid.net/specs/openid-connect-core-1_0.html#HybridIDTValidation2)）。

**標準仕様（ID Token の Claim の値）**：ID Token の `iss` は、クエリとフラグメントを含まない HTTPS URL です。
`sub` は Issuer 内で局所的に一意で、再割当てされない識別子であり、255 ASCII 文字を超えることは **MUST NOT** です。
`aud` は Relying Party の OAuth 2.0 `client_id` を含むことが **MUST** です。
認証要求で `nonce` を送った場合、ID Token に `nonce` があり、値が一致することを確かめます（**MUST**）（[OpenID Connect Core 1.0 §2、§3.1.3.7](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)）。

**標準仕様（OpenID Provider Metadata）**：OpenID Connect Discovery 1.0 §3 では `issuer`、`authorization_endpoint`、`jwks_uri`、`response_types_supported`、`subject_types_supported`、`id_token_signing_alg_values_supported` を **REQUIRED** とします。
`token_endpoint` も、Implicit Flow だけを使う場合を除いて **REQUIRED** です。
`issuer` はクエリ・フラグメントを含まない HTTPS URL で、発行する ID Token の `iss` と一致することが **MUST** です。
`authorization_endpoint`、`token_endpoint`、`jwks_uri` は HTTPS URL です。
`id_token_signing_alg_values_supported` は `RS256` を含むことが **MUST** です（[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)）。

**標準仕様（OpenID Provider Metadata の検証）**：Provider Metadata の `issuer` は、取得に使った Issuer の URL と同一であることが **MUST** です。
検証に失敗した情報を使うことは **MUST NOT** です（[OpenID Connect Discovery 1.0 §4.3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderConfigurationValidation)）。

**適用範囲**：JWT / JWS / JWK の共通規則と、OpenID Connect の ID Token および認証フローを前提とする追加規則を区別します。例えば、登録済み `client_id` を Audience に含める要件は OIDC ID Token の検証規則であり、JWT 一般の `aud` の形式要件ではありません。認証要求で送った `nonce` の照合も、認証フローを前提とする規則です。一方、`aud` がある JWT の受信者照合や、Issuer と検証鍵を適切に結び付ける必要性は、JWT の一般的な規則とセキュリティ上の考慮事項に基づきます（[RFC 7519 §4.1.3](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.3)、[RFC 8725 §3.8–§3.9](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.8)、[OpenID Connect Core 1.0 §3.1.3.7](https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation)）。

**OpenID Provider Metadata の適用範囲**：Discovery の必須項目は OpenID Provider Metadata に対する要件です。
必須項目が欠けた Metadata は Discovery の要件を満たしません。
ただし、この欠落だけでは、その Issuer が発行した JWT の署名や Claim の有効性は決まりません。
署名と Claim の検証は、JWT / JWS の規則と、ID Token であれば OpenID Connect Core の規則に従うためです。

**OIDC ID Token 以外の JWT**：OpenID Connect Core が ID Token に定める必須 Claim や Issuer の URL 形式は、OIDC ID Token 以外の JWT に一律に適用されるものではありません（[OpenID Connect Core 1.0 §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)）。

## 6. JWT Security Considerations

**標準仕様**：RFC 8725 は、JWT の発行と検証に関して次の点を定めています。アルゴリズムの規定は、この文書の §2.1 に置きます。

- **UTF-8**：Header と Claims Set の JSON には UTF-8 を使い、ほかの Unicode 符号化を認めないことが **MUST**（[§3.7](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.7)）
- **Issuer と Subject**：`iss` があれば、使う鍵がその Issuer に属することを確かめ、属さなければ拒否することが **MUST**。`sub` があれば、アプリケーションにとって有効な値か確かめ、無効なら拒否することが **MUST**（[§3.8](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.8)）
- **Audience**：同じ Issuer が複数の受信者向けに JWT を発行し得る場合、JWT は `aud` を持つことが **MUST**。受信者は `aud` を検証し、欠落や不一致なら拒否することが **MUST**（[§3.9](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.9)）
- **受信した値**：`kid` はインジェクションを招かないよう検証または無害化する。`jku` と `x5u` の URL をそのままたどらず、SSRF を防ぐことが **SHOULD**（[§3.10](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.10)）
- **JWT の種類の区別**：同じ Issuer が複数の種類の JWT を発行し得る場合、検証規則は互いに排他的であることが **MUST**。手段には `typ`、必須 Claim、Header、鍵、`aud`、Issuer の違いがある。新しい用途では `typ` による明示が **RECOMMENDED**（[§3.11–§3.12](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.11)）

## 7. Sources

- [RFC 7159 — The JavaScript Object Notation (JSON) Data Interchange Format](https://www.rfc-editor.org/rfc/rfc7159.html)（2014、§8.3）
- [RFC 7515 — JSON Web Signature (JWS)](https://www.rfc-editor.org/rfc/rfc7515.html)（2015、§3–§6）
- [RFC 7517 — JSON Web Key (JWK)](https://www.rfc-editor.org/rfc/rfc7517.html)（2015、§4–§5）
- [RFC 7518 — JSON Web Algorithms (JWA)](https://www.rfc-editor.org/rfc/rfc7518.html)（2015、§3.3、§3.5、§6.3）
- [RFC 7519 — JSON Web Token (JWT)](https://www.rfc-editor.org/rfc/rfc7519.html)（2015、§2、§4、§7.3）
- [RFC 7797 — JSON Web Signature (JWS) Unencoded Payload Option](https://www.rfc-editor.org/rfc/rfc7797.html)（2016、§6–§7。RFC 7519 の更新）
- [RFC 8725 — JSON Web Token Best Current Practices](https://www.rfc-editor.org/rfc/rfc8725.html)（2020、§3。RFC 7519 の更新）
- [OpenID Connect Core 1.0 incorporating errata set 2](https://openid.net/specs/openid-connect-core-1_0.html)（2023-12-15、§2、§3.1.3.7、§3.2.2.11、§3.3.2.12、§3.3.3.7、§8、§10.1、§14）
- [OpenID Connect Discovery 1.0 incorporating errata set 2](https://openid.net/specs/openid-connect-discovery-1_0.html)（2023-12-15、§3、§4.3、§5）
