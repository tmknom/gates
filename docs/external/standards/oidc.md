# OpenID Connect Standards

この文書は、OpenID Connect Core と OpenID Connect Discovery の標準仕様から、ID Token、署名鍵、Discovery の検証に関係する規定を整理したものです。標準仕様の代替ではなく、規定の適用対象と条件はリンク先の原文を優先します。

- **対象**：ID Token の Claim と検証、署名鍵と JWK Set の用途、文字列比較、OpenID Provider Metadata に関係する規定
- **対象外**：OAuth 2.0 の認可フロー全体、JWE による暗号化

仕様の **MUST / SHOULD / MAY** は原文の規範的強度を表します。OpenID Connect Core / Discovery の規定は、JWT 一般の仕様とは適用対象が異なります。特に ID Token や OpenID Connect の認証フローに限定される条件は、この文書の [§1](#1-openid-connect-and-discovery) で区別します。

## 1. OpenID Connect and Discovery

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

## 2. Signing Keys and JWK Set

**標準仕様（OIDC 固有）**：OpenID Connect Core §10.1 は、RSA または ECDSA の署名者に、公開した JWK Set の公開鍵に対応する秘密鍵を使うことを求めます。
参照する JWK Set に複数の鍵があれば、JOSE Header に `kid` を指定することが **MUST** で、鍵の用途は署名に対応していることが **MUST** です。
したがって、RFC 7515 の `kid` が任意という規定だけで、OIDC のすべてのケースにおける省略を許容できるとはいえません（[OpenID Connect Core 1.0 §10.1](https://openid.net/specs/openid-connect-core-1_0.html#Signing)）。

**標準仕様（OIDC 固有）**：OpenID Connect Discovery は、同じ JWK Set に署名用と暗号化用の鍵の両方を公開するとき、**すべての鍵** に `use` を要求します。したがって RFC 7517 の「任意」だけから、OIDC の JWK Set でも常に欠落を許容できると結論づけられません。
JWK Set に秘密鍵と対称鍵の値を含めることは **MUST NOT** です（[OpenID Connect Discovery 1.0 §3 `jwks_uri`](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)）。

## 3. String Comparison

**標準仕様（OIDC 固有）**：OpenID Connect のメッセージの文字列比較では、Unicode 正規化が **MUST NOT** で、コードポイント単位の等価比較が **MUST** です（[OpenID Connect Core 1.0 §14](https://openid.net/specs/openid-connect-core-1_0.html#StringOps)、[OpenID Connect Discovery 1.0 §5](https://openid.net/specs/openid-connect-discovery-1_0.html#StringOps)）。

## 4. References

- [OpenID Connect Core 1.0 incorporating errata set 2](https://openid.net/specs/openid-connect-core-1_0.html)（2023-12-15、§2、§3.1.3.7、§3.2.2.11、§3.3.2.12、§3.3.3.7、§8、§10.1、§14）
- [OpenID Connect Discovery 1.0 incorporating errata set 2](https://openid.net/specs/openid-connect-discovery-1_0.html)（2023-12-15、§3、§4.3、§5）
