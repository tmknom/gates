# OpenID Connect Discovery 1.0

この文書は、OpenID Connect Discovery から、OpenID Provider Metadata とその取得と検証に関係する規定を、ID の付いた条項に分けて整理したものです。標準仕様の代替ではなく、規定の適用対象と条件はリンク先の原文を優先します。

- **対象**：OpenID Provider Metadata のうち `issuer`、`authorization_endpoint`、`token_endpoint`、`jwks_uri`、`response_types_supported`、`subject_types_supported`、`id_token_signing_alg_values_supported` と追加の Metadata（§3）、設定の取得と検証（§4、§4.1–§4.3）、文字列の操作（§5）、TLS と成りすましへの対策（§7.1、§7.2）
- **対象外**：Issuer の WebFinger による発見（§2）、対象に挙げたもの以外の OpenID Provider Metadata と CORS（§3、§4）、実装の考慮事項（§6）、そのほかのセキュリティの考慮事項

各条項の Level は、原文が使う [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119.html) の語か、RFC 2119 の語を使わない定義を表す Definition です。
OpenID Connect Discovery は、RFC 2119 の語を大文字に限る旨を書いていません。条項にした節の小文字の「may」は可能性を表す言い回しで、規範ではありません。

Actor は、[OpenID Connect Core 1.0](oidc-core.md) と同じく、OpenID Connect Core §1.2 の定義に基づいて Relying Party と OpenID Provider に統一します。原文の Client と RP は Relying Party、OP は OpenID Provider です。

OpenID Connect Discovery の規定は、OpenID Provider Metadata とその取得に対する要件で、ID Token の `iss` と `issuer` の一致（OIDCD-ISSUER-ID-TOKEN-ISS、OIDCD-CONFIG-ISSUER-ID-TOKEN）を除き、JWT の署名と Claim の検証の規則は定めません。必須の Metadata が欠けていれば Discovery の要件を満たしませんが、その OpenID Provider が発行した JWT の署名と Claim の検証は、JWS、JWK、JWT の標準仕様（[RFC 7515 JSON Web Signature](rfc7515.md)、[RFC 7519 JSON Web Token](rfc7519.md) など）の規則と、ID Token であれば [OpenID Connect Core 1.0](oidc-core.md) の規則に従います。

## 1. OpenID Provider Metadata

OpenID Connect Discovery §3 は、OpenID Provider が自身の設定を表す Metadata を定めています。この節の条項は Metadata の値の要件で、Actor は Metadata を公開する OpenID Provider です。原文の多くは値に課す要件として書かれ、主体を書いていません。

### OIDCD-ISSUER-REQUIRED `issuer` の存在

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`REQUIRED. URL using the https scheme with no query or fragment components that the OP asserts as its Issuer Identifier.`
- **Level**：REQUIRED
- **Actor**：OpenID Provider

### OIDCD-ISSUER-FORMAT `issuer` の形式

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`REQUIRED. URL using the https scheme with no query or fragment components that the OP asserts as its Issuer Identifier.`
- **Level**：Definition

### OIDCD-ISSUER-WEBFINGER `issuer` と WebFinger の `issuer` の同一性

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`If Issuer discovery is supported (see Section 2), this value MUST be identical to the issuer value returned by WebFinger.`
- **Level**：MUST
- **Actor**：OpenID Provider
- **Condition**：Issuer の発見（§2）に対応する

### OIDCD-ISSUER-ID-TOKEN-ISS `issuer` と ID Token の `iss` の同一性

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`This also MUST be identical to the iss Claim value in ID Tokens issued from this Issuer.`
- **Level**：MUST
- **Actor**：OpenID Provider
- **Condition**：ID Token である

受け取った側の検証は OIDCD-CONFIG-ISSUER-ID-TOKEN が定めます。

### OIDCD-AUTHORIZATION-ENDPOINT-REQUIRED `authorization_endpoint` の存在

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`REQUIRED. URL of the OP's OAuth 2.0 Authorization Endpoint [OpenID.Core].`
- **Level**：REQUIRED
- **Actor**：OpenID Provider

### OIDCD-AUTHORIZATION-ENDPOINT-HTTPS `authorization_endpoint` の `https`

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`This URL MUST use the https scheme and MAY contain port, path, and query parameter components.`
- **Level**：MUST
- **Actor**：OpenID Provider

### OIDCD-AUTHORIZATION-ENDPOINT-COMPONENTS `authorization_endpoint` のポート、パス、クエリ

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`This URL MUST use the https scheme and MAY contain port, path, and query parameter components.`
- **Level**：MAY
- **Actor**：OpenID Provider

### OIDCD-TOKEN-ENDPOINT-REQUIRED `token_endpoint` の存在

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`This is REQUIRED unless only the Implicit Flow is used.`
- **Level**：REQUIRED
- **Actor**：OpenID Provider
- **Condition**：Implicit Flow だけを使うのではない

### OIDCD-TOKEN-ENDPOINT-HTTPS `token_endpoint` の `https`

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`This URL MUST use the https scheme and MAY contain port, path, and query parameter components.`
- **Level**：MUST
- **Actor**：OpenID Provider

原文の主語「This URL」は `token_endpoint` の値なので、この条項は `token_endpoint` がある Metadata に適用されます。次の条項も同じです。

### OIDCD-TOKEN-ENDPOINT-COMPONENTS `token_endpoint` のポート、パス、クエリ

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`This URL MUST use the https scheme and MAY contain port, path, and query parameter components.`
- **Level**：MAY
- **Actor**：OpenID Provider

### OIDCD-JWKS-URI-REQUIRED `jwks_uri` の存在

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`REQUIRED. URL of the OP's JWK Set [JWK] document, which MUST use the https scheme.`
- **Level**：REQUIRED
- **Actor**：OpenID Provider

### OIDCD-JWKS-URI-HTTPS `jwks_uri` の `https`

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`REQUIRED. URL of the OP's JWK Set [JWK] document, which MUST use the https scheme.`
- **Level**：MUST
- **Actor**：OpenID Provider

### OIDCD-JWKS-SIGNING-KEYS `jwks_uri` の JWK Set の署名鍵

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`This contains the signing key(s) the RP uses to validate signatures from the OP.`
- **Level**：Definition

### OIDCD-JWKS-ENCRYPTION-KEYS `jwks_uri` の JWK Set の暗号化鍵

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`The JWK Set MAY also contain the Server's encryption key(s), which are used by RPs to encrypt requests to the Server.`
- **Level**：MAY
- **Actor**：OpenID Provider

### OIDCD-JWKS-USE-REQUIRED 署名鍵と暗号化鍵を公開する JWK Set の `use`

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`When both signing and encryption keys are made available, a use (public key use) parameter value is REQUIRED for all keys in the referenced JWK Set to indicate each key's intended usage.`
- **Level**：REQUIRED
- **Actor**：OpenID Provider
- **Condition**：`jwks_uri` の JWK Set に署名鍵と暗号化鍵の両方がある

RFC 7517 の `use` は、アプリケーションが求めない限り OPTIONAL です（[RFC 7517 JSON Web Key](rfc7517.md) の RFC7517-USE-OPTIONAL）。この条件では、署名鍵だけでなく、JWK Set のすべての鍵に `use` が必要です。

### OIDCD-JWKS-KEY-DUAL-USE 一つの鍵の署名と暗号化の兼用

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`Although some algorithms allow the same key to be used for both signatures and encryption, doing so is NOT RECOMMENDED, as it is less secure.`
- **Level**：NOT RECOMMENDED
- **Actor**：OpenID Provider

### OIDCD-JWKS-X5C JWK Set の `x5c`

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`The JWK x5c parameter MAY be used to provide X.509 representations of keys provided.`
- **Level**：MAY
- **Actor**：OpenID Provider

### OIDCD-JWKS-X5C-BARE-KEY-PRESENT `x5c` を使う JWK の鍵の値の存在

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`When used, the bare key values MUST still be present and MUST match those in the certificate.`
- **Level**：MUST
- **Actor**：OpenID Provider
- **Condition**：JWK に `x5c` がある

鍵の値は、RSA の鍵なら `n` と `e` のような、鍵の種類ごとのメンバーです。

### OIDCD-JWKS-X5C-BARE-KEY-MATCH `x5c` を使う JWK の鍵の値と証明書の一致

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`When used, the bare key values MUST still be present and MUST match those in the certificate.`
- **Level**：MUST
- **Actor**：OpenID Provider
- **Condition**：JWK に `x5c` がある

### OIDCD-JWKS-NO-PRIVATE JWK Set の秘密鍵と対称鍵の値の禁止

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`The JWK Set MUST NOT contain private or symmetric key values.`
- **Level**：MUST NOT
- **Actor**：OpenID Provider

秘密鍵と対称鍵の値を表すメンバーは、鍵の種類の定義が定めます（[RFC 7518 JSON Web Algorithms](rfc7518.md) の RFC7518-EC-PRIVATE-D、RFC7518-RSA-PRIVATE-MEMBERS、RFC7518-OCT-KEY-VALUE、[RFC 9964 ML-DSA for JOSE and COSE](rfc9964.md) の RFC9964-AKP-PRIV-PUBLIC-KEY）。

### OIDCD-RESPONSE-TYPES-REQUIRED `response_types_supported` の存在

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`REQUIRED. JSON array containing a list of the OAuth 2.0 response_type values that this OP supports.`
- **Level**：REQUIRED
- **Actor**：OpenID Provider

### OIDCD-RESPONSE-TYPES-DYNAMIC Dynamic OpenID Provider が対応する Response Type

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`Dynamic OpenID Providers MUST support the code, id_token, and the id_token token Response Type values.`
- **Level**：MUST
- **Actor**：OpenID Provider
- **Condition**：Dynamic OpenID Provider である

Dynamic OpenID Provider は、事前の関係なしに Relying Party が動的に使える OpenID Provider です（Core §15）。

### OIDCD-SUBJECT-TYPES-REQUIRED `subject_types_supported` の存在

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`REQUIRED. JSON array containing a list of the Subject Identifier types that this OP supports.`
- **Level**：REQUIRED
- **Actor**：OpenID Provider

有効な値には `pairwise` と `public` があります（原文）。

### OIDCD-ID-TOKEN-SIGNING-ALG-REQUIRED `id_token_signing_alg_values_supported` の存在

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`REQUIRED. JSON array containing a list of the JWS signing algorithms (alg values) supported by the OP for the ID Token to encode the Claims in a JWT [JWT].`
- **Level**：REQUIRED
- **Actor**：OpenID Provider

### OIDCD-ID-TOKEN-SIGNING-ALG-RS256 `id_token_signing_alg_values_supported` の `RS256`

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`The algorithm RS256 MUST be included.`
- **Level**：MUST
- **Actor**：OpenID Provider

### OIDCD-ID-TOKEN-SIGNING-ALG-NONE-SUPPORT `id_token_signing_alg_values_supported` の `none` への対応

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`The value none MAY be supported but MUST NOT be used unless the Response Type used returns no ID Token from the Authorization Endpoint (such as when using the Authorization Code Flow).`
- **Level**：MAY
- **Actor**：OpenID Provider

### OIDCD-ID-TOKEN-SIGNING-ALG-NONE-USE ID Token の `none` の使用の制限

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`The value none MAY be supported but MUST NOT be used unless the Response Type used returns no ID Token from the Authorization Endpoint (such as when using the Authorization Code Flow).`
- **Level**：MUST NOT
- **Actor**：OpenID Provider
- **Condition**：ID Token で、Response Type が Authorization Endpoint から ID Token を返す

[OpenID Connect Core 1.0](oidc-core.md) の OIDCC-ALG-NONE は、Client が登録時に `none` の使用を明示的に求めたことも条件に加えています。

### OIDCD-METADATA-ADDITIONAL 追加の OpenID Provider Metadata

- **Source**：[OpenID Connect Discovery 1.0 §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)
- **Quote**：`Additional OpenID Provider Metadata parameters MAY also be used.`
- **Level**：MAY
- **Actor**：OpenID Provider

## 2. Obtaining and Validating the Configuration

### OIDCD-CONFIG-PATH OpenID Provider の設定の文書の場所

- **Source**：[OpenID Connect Discovery 1.0 §4](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderConfig)
- **Quote**：`OpenID Providers supporting Discovery MUST make a JSON document available at the path formed by concatenating the string /.well-known/openid-configuration to the Issuer.`
- **Level**：MUST
- **Actor**：OpenID Provider
- **Condition**：OpenID Provider が Discovery に対応する

### OIDCD-CONFIG-DOCUMENT `openid-configuration` の文書の準拠

- **Source**：[OpenID Connect Discovery 1.0 §4](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderConfig)
- **Quote**：`openid-configuration MUST point to a JSON document compliant with this specification and MUST be returned using the application/json content type.`
- **Level**：MUST
- **Actor**：OpenID Provider
- **Condition**：OpenID Provider が Discovery に対応する

原文は文書に課す要件として書き、主体を書いていません。文書を公開するのは OpenID Provider なので、Actor を OpenID Provider とします。次の条項も同じです。

### OIDCD-CONFIG-CONTENT-TYPE `openid-configuration` の `application/json`

- **Source**：[OpenID Connect Discovery 1.0 §4](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderConfig)
- **Quote**：`openid-configuration MUST point to a JSON document compliant with this specification and MUST be returned using the application/json content type.`
- **Level**：MUST
- **Actor**：OpenID Provider
- **Condition**：OpenID Provider が Discovery に対応する

### OIDCD-CONFIG-GET 設定の HTTP GET による取得

- **Source**：[OpenID Connect Discovery 1.0 §4.1](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderConfigurationRequest)
- **Quote**：`An OpenID Provider's configuration information MUST be retrieved using an HTTP GET request at the previously specified path.`
- **Level**：MUST
- **Actor**：Relying Party

原文は受動態で主体を書いていません。§4.1 の例は要求を送る主体を「The RP」と書くため、Actor を Relying Party とします。次の条項も同じです。

### OIDCD-CONFIG-PATH-SLASH パスを持つ Issuer の末尾の `/` の除去

- **Source**：[OpenID Connect Discovery 1.0 §4.1](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderConfigurationRequest)
- **Quote**：`If the Issuer value contains a path component, any terminating / MUST be removed before appending /.well-known/openid-configuration.`
- **Level**：MUST
- **Actor**：Relying Party
- **Condition**：Issuer の値がパスを持つ

Issuer が `https://example.com/issuer1` なら、要求のパスは `/issuer1/.well-known/openid-configuration` です（原文）。

### OIDCD-CONFIG-RESPONSE 設定の成功の応答

- **Source**：[OpenID Connect Discovery 1.0 §4.2](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderConfigurationResponse)
- **Quote**：`A successful response MUST use the 200 OK HTTP status code and return a JSON object using the application/json content type that contains a set of Claims as its members that are a subset of the Metadata values defined in Section 3.`
- **Level**：MUST
- **Actor**：OpenID Provider

原文は応答に課す要件として書き、主体を書いていません。応答を返すのは OpenID Provider なので、Actor を OpenID Provider とします。この小節の残りの応答の条項も同じです。

### OIDCD-CONFIG-OTHER-CLAIMS 設定の応答のほかの Claim

- **Source**：[OpenID Connect Discovery 1.0 §4.2](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderConfigurationResponse)
- **Quote**：`Other Claims MAY also be returned.`
- **Level**：MAY
- **Actor**：OpenID Provider

### OIDCD-CONFIG-EMPTY-OMIT 要素のない Claim の省略

- **Source**：[OpenID Connect Discovery 1.0 §4.2](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderConfigurationResponse)
- **Quote**：`Claims that return multiple values are represented as JSON arrays. Claims with zero elements MUST be omitted from the response.`
- **Level**：MUST
- **Actor**：OpenID Provider

### OIDCD-CONFIG-VALIDATION-ABORT 検証に失敗した情報を必要とする操作の中止

- **Source**：[OpenID Connect Discovery 1.0 §4.3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderConfigurationValidation)
- **Quote**：`If any of the validation procedures defined in this specification fail, any operations requiring the information that failed to correctly validate MUST be aborted and the information that failed to validate MUST NOT be used.`
- **Level**：MUST
- **Actor**：Relying Party
- **Condition**：OpenID Connect Discovery が定める検証のどれかに失敗した

原文は受動態で主体を書いていません。§7.2 は設定の検証を行う主体を「RPs」と書くため、Actor を Relying Party とします。この小節の残りの条項も同じです。

### OIDCD-CONFIG-VALIDATION-UNUSED 検証に失敗した情報の不使用

- **Source**：[OpenID Connect Discovery 1.0 §4.3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderConfigurationValidation)
- **Quote**：`If any of the validation procedures defined in this specification fail, any operations requiring the information that failed to correctly validate MUST be aborted and the information that failed to validate MUST NOT be used.`
- **Level**：MUST NOT
- **Actor**：Relying Party
- **Condition**：OpenID Connect Discovery が定める検証のどれかに失敗した

### OIDCD-CONFIG-ISSUER-MATCH `issuer` と取得に使った Issuer の URL の同一性

- **Source**：[OpenID Connect Discovery 1.0 §4.3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderConfigurationValidation)
- **Quote**：`The issuer value returned MUST be identical to the Issuer URL that was used as the prefix to /.well-known/openid-configuration to retrieve the configuration information.`
- **Level**：MUST
- **Actor**：Relying Party

§7.2 の「To prevent this, RPs MUST ensure that the Issuer URL they are using for the Configuration Request exactly matches the value of the issuer Claim in the OP Metadata document received by the RP and that this also exactly matches the iss Claim value in ID Tokens that are supposed to be from that Issuer.」は、この条項と OIDCD-CONFIG-ISSUER-ID-TOKEN を言い換えたもので、別の条項にしていません。「this」は、攻撃者が正規の OpenID Provider の Issuer の URL を `issuer` に書き、自身のエンドポイントと署名鍵を載せた文書を公開する成りすましです。比較は §5 の規則（OIDCD-STRING-UNESCAPE、OIDCD-STRING-NO-NORMALIZATION、OIDCD-STRING-CODE-POINT）に従います。

### OIDCD-CONFIG-ISSUER-ID-TOKEN `issuer` と ID Token の `iss` の同一性の検証

- **Source**：[OpenID Connect Discovery 1.0 §4.3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderConfigurationValidation)
- **Quote**：`This MUST also be identical to the iss Claim value in ID Tokens issued from this Issuer.`
- **Level**：MUST
- **Actor**：Relying Party
- **Condition**：ID Token である

§7.2 の Relying Party への MUST（OIDCD-CONFIG-ISSUER-MATCH に引用）が、同じ規範を言い換えています。発行の側の規定は OIDCD-ISSUER-ID-TOKEN-ISS です。

## 3. String Operations

OpenID Connect Discovery §5 は、OpenID Connect Core §14 と同じ比較の手順を定めています。原文は比較の主体を書いていません。Discovery §6 は、Discovery を実装する Relying Party と OpenID Provider の両方が機能を実装するとし、§7.1 はその主体を「Implementations」と呼ぶため、この節の条項の Actor を Implementation とします。

### OIDCD-STRING-UNESCAPE 比較の前の JSON のエスケープの除去

- **Source**：[OpenID Connect Discovery 1.0 §5](https://openid.net/specs/openid-connect-discovery-1_0.html#StringOps)
- **Quote**：`Therefore, comparisons between JSON strings and other Unicode strings MUST be performed as specified below: Remove any JSON applied escaping to produce an array of Unicode code points.`
- **Level**：MUST
- **Actor**：Implementation

### OIDCD-STRING-NO-NORMALIZATION Unicode 正規化の禁止

- **Source**：[OpenID Connect Discovery 1.0 §5](https://openid.net/specs/openid-connect-discovery-1_0.html#StringOps)
- **Quote**：`Unicode Normalization [USA15] MUST NOT be applied at any point to either the JSON string or to the string it is to be compared against.`
- **Level**：MUST NOT
- **Actor**：Implementation

### OIDCD-STRING-CODE-POINT コードポイント単位の等価比較

- **Source**：[OpenID Connect Discovery 1.0 §5](https://openid.net/specs/openid-connect-discovery-1_0.html#StringOps)
- **Quote**：`Comparisons between the two strings MUST be performed as a Unicode code point to code point equality comparison.`
- **Level**：MUST
- **Actor**：Implementation

## 4. TLS and Impersonation

### OIDCD-TLS-SUPPORT TLS への対応

- **Source**：[OpenID Connect Discovery 1.0 §7.1](https://openid.net/specs/openid-connect-discovery-1_0.html#TLSRequirements)
- **Quote**：`Implementations MUST support TLS.`
- **Level**：MUST
- **Actor**：Implementation

### OIDCD-TLS-BCP195 BCP 195 の指針

- **Source**：[OpenID Connect Discovery 1.0 §7.1](https://openid.net/specs/openid-connect-discovery-1_0.html#TLSRequirements)
- **Quote**：`Implementations SHOULD follow the guidance in BCP 195 [RFC8996] [RFC9325], which provides recommendations and requirements for improving the security of deployed services that use TLS.`
- **Level**：SHOULD
- **Actor**：Implementation

### OIDCD-TLS-CONFIDENTIALITY 機密性と完全性を提供する暗号スイートの TLS

- **Source**：[OpenID Connect Discovery 1.0 §7.1](https://openid.net/specs/openid-connect-discovery-1_0.html#TLSRequirements)
- **Quote**：`To protect against information disclosure and tampering, confidentiality protection MUST be applied using TLS with a ciphersuite that provides confidentiality and integrity protection.`
- **Level**：MUST
- **Actor**：Implementation

原文は受動態で主体を書いていません。§7.1 の前の文の主語が「Implementations」なので、Actor を Implementation とします。次の条項も同じです。

### OIDCD-TLS-CERTIFICATE TLS のサーバー証明書の確認

- **Source**：[OpenID Connect Discovery 1.0 §7.1](https://openid.net/specs/openid-connect-discovery-1_0.html#TLSRequirements)
- **Quote**：`Whenever TLS is used, a TLS server certificate check MUST be performed, per RFC 6125 [RFC6125].`
- **Level**：MUST
- **Actor**：Implementation
- **Condition**：TLS を使う

### OIDCD-CONFIG-TLS-CHECK 設定の要求での TLS の証明書の確認

- **Source**：[OpenID Connect Discovery 1.0 §7.2](https://openid.net/specs/openid-connect-discovery-1_0.html#Impersonation)
- **Quote**：`TLS certificate checking MUST be performed by the RP, as described in Section 7.1, when making an OpenID Provider Configuration Request.`
- **Level**：MUST
- **Actor**：Relying Party
- **Condition**：Relying Party が OpenID Provider の設定を要求する

サーバー証明書が Issuer の URL に対して有効であることを確かめると、中間者攻撃と DNS を使った攻撃を防げます。防げないと、Relying Party が攻撃者の鍵とエンドポイントを使わされ、正規の Issuer に成りすまされるおそれがあります（原文）。

## 5. References

- [RFC 2119 — Key words for use in RFCs to Indicate Requirement Levels](https://www.rfc-editor.org/rfc/rfc2119.html)（1997）
- [OpenID Connect Core 1.0 incorporating errata set 2](https://openid.net/specs/openid-connect-core-1_0.html)（2023、§1.2、§14、§15）
- [OpenID Connect Discovery 1.0 incorporating errata set 2](https://openid.net/specs/openid-connect-discovery-1_0.html)（2023、§3、§4、§4.1–§4.3、§5、§6、§7.1、§7.2）
