# GATES OIDC JWT Signature

本ドキュメントは、GATES 用 OIDC JWT の直列化、JOSE Header、署名の検証鍵に GATES が課す受理条件を定めます。
Claims Set の受理条件は [GATES OIDC JWT Claims](gates-oidc-jwt-claims.md) に定め、規則の ID、根拠の区分、受理の分類もそちらに従います。
各規則の ID は同じ `GOJ-` の接頭辞で始まります。

Discovery / JWKS の取得時刻、キャッシュの存続、再取得の上限、タイムアウト、応答サイズ、失敗時の戻り値は本書の受理条件ではなく、検証器のコンポーネント設計が扱います。

## GOJ-SERIALIZATION JWS と JSON の基本条件

**標準仕様**：JWT は JWS / JWE の形式で表せますが、GATES が検証する GitHub OIDC JWT は署名付き JWS Compact Serialization に限ります。JWS は Protected Header、Payload、Signature の3部分を `.` で連結し、Base64URL で表します（[RFC 7515 §3.1、§5.2](https://www.rfc-editor.org/rfc/rfc7515.html#section-5.2)、[RFC 7519 §1](https://www.rfc-editor.org/rfc/rfc7519.html#section-1)）。Header と Claims Set に UTF-8 を使うことは RFC 8725 §3.7 の MUST です。

**GATES の受理条件**：JWS Compact Serialization だけを受理し、3部分以外の形、JSON Serialization、Base64URL 以外の文字、余分な padding・空白は拒否します。Header / Claims Set で、エスケープ解除後に同一となる JSON 名の重複や不正な UTF-8 も拒否します。未対応の JWE は受理しません。

JWT の生の文字列に CR / LF を含む場合も拒否します。JWS Compact Serialization の符号化された構成要素に改行を含めることはできず、検証ライブラリが読み飛ばす挙動を受理条件の代用にしません。

## GOJ-HEADER JOSE Header

| Header | 標準仕様 | GATES の受理条件・扱い |
|---|---|---|
| `alg` | JWS で必須。実際の署名方式と一致 | 必須。`RS256` のみ。`none`、`PS256` 等は拒否 |
| `kid` | JWS 一般では任意。OIDC Core §10.1 では JWKS に複数鍵があるとき必須 | 必須。JWKS 内の使用可能な鍵を選択するために利用。値は大文字小文字を区別 |
| `typ` | 任意。JWT の種類識別に使える | 存在を必須にしない。署名鍵の取得先を決めない |
| `cty` | RFC 7519 §5.2 で Nested JWT を示す場合は `JWT`（大小文字を区別しない）が必要 | GATES は Nested JWT に対応せず、`cty` が存在する JWT は値を問わず拒否する |
| `crit` | 未対応の拡張があれば無効 | GATES は拡張を実装しないため、存在すれば拒否 |
| `b64` | RFC 7797 は `b64:false` を JWT で禁止 | `true` / `false` とも GATES が扱う JWT プロファイルでは受理しない |
| `jku`, `jwk`, `x5u`, `x5c`, `x5t`, `x5t#S256` | 署名鍵情報に関係する任意の Header | 存在を拒否理由にせず、鍵の取得先・検証鍵選択の根拠にしない |
| その他の未理解 Header | 一般には無視することが MUST | 無視する。未知であることだけでは拒否しない |

`cty` を一律拒否するのは、Nested JWT に対応しない GATES の JWS プロファイルの追加制約です。`kid` は外部 URL やファイルパスとして解釈せず、固定した Issuer の JWKS 内で鍵を引くためだけに使います。`kid` を特定の UUID / 16進数字形式に限定しません（実測の JWKS でも形式は混在）。`crit` が空・不正であっても受理せず、未対応の拡張が意味する処理を無視しません。`b64:true` の拒否は標準による一律禁止ではなく GATES の追加制約です（[RFC 7515 §4](https://www.rfc-editor.org/rfc/rfc7515.html#section-4)、[RFC 7797 §6–7](https://www.rfc-editor.org/rfc/rfc7797.html#section-7)）。

## GOJ-DISCOVERY Discovery と鍵の取得先

GitHub の Discovery 公開値は `issuer`、`jwks_uri`、`subject_types_supported`、`response_types_supported`、`claims_supported`、`id_token_signing_alg_values_supported`、`scopes_supported` で、`authorization_endpoint` と `token_endpoint` を持ちません（[調査記録](../research/github/oidc-discovery.md)）。

**標準仕様との境界**：OpenID Connect Discovery §3 は `authorization_endpoint` を REQUIRED とし、`token_endpoint` は Implicit Flow だけを使用する場合を除いて REQUIRED とします。この GitHub 公開値は完全な OpenID Provider Metadata の要求を満たしません。ただし、Metadata の不足だけで個々の JWT の署名・Claim が不正だと断定することはできません（[OpenID Connect Discovery §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)、[JWT Constraints §5](../spec/jwt-constraints.md#5-openid-connect-and-discovery)）。

**GATES の設計**：署名検証鍵は、[GOJ-ISS](gates-oidc-jwt-claims.md#goj-iss-iss) の固定 Issuer の検証済み Discovery / JWKS だけから得ます。
固定 Issuer の Discovery を HTTPS で取得し、**一致を確認した `issuer` と JWKS の取得先 `jwks_uri`** だけを使用します。
Discovery に異なる `issuer` があればその Metadata を利用しません。
認証 Endpoint、`client_id`、`nonce` を前提にした通常の OpenID Connect 認証フローは実行しません。
JWKS の URL は Issuer と同じ HTTPS のホスト・ポートに制限し、別ドメインへリダイレクトしません。
JWT の Header やリクエストが任意の鍵取得 URL を指定することも許しません。

完全な Discovery クライアントとして標準準拠しているとは主張しません。

## GOJ-JWK JWK の受理条件

**標準仕様**：JWK Set は `keys` 配列を持つ JSON オブジェクトです。RSA 公開鍵では `kty=RSA`、`n`、`e` を用います。`use` と `key_ops` の併用・整合性、`RS256` の RSA 鍵長については [RFC 7517 §4–5](https://www.rfc-editor.org/rfc/rfc7517.html#section-4)、[RFC 7518 §3.3・§6.3](https://www.rfc-editor.org/rfc/rfc7518.html#section-6.3) に従います。OpenID Connect Discovery §3 は Provider に対し、JWKS に秘密鍵・対称鍵の値を含めることを **MUST NOT** とします（[Discovery §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)）。GATES では、この違反を含む JWKS 全体を拒否する追加制約を採用します（[GOJ-JWKS](#goj-jwks-jwk-set-の拒否とエラー分類)）。

**GATES の鍵採用条件**：次の条件をすべて満たし、RSA 公開鍵として検証処理に渡せる JWK を **使用可能な鍵** とします。これは署名の一致を意味しません。個別条件に適合しない JWK は **除外した JWK** とし、保持しません。

- `kty=RSA`。`kid` は空でない JSON 文字列であり、`n` と `e` は正しい Base64urlUInt として復号できる。
- RSA の法 `n` は2048 bit 以上の正の奇数、公開指数 `e` は3以上の奇数。`e` は `2^31 − 1` 以下とする。奇偶と下限は RSA 公開鍵としての妥当性を確保し、上限は署名の検証で拒否される鍵を保持して署名の不一致に見えないようにする GATES の追加制約であり、RFC 7518 の `n` / `e` の Base64urlUInt という表現要件とは区別する。
- `use` が存在すれば `sig`、`key_ops` が存在すれば文字列の配列で `verify` を含み、重複がない。`use` と `key_ops` が両方存在する場合は、`use=sig` と整合させるため、`key_ops` の値を `sign` と `verify` に限る（`["verify"]` と `["sign", "verify"]` を受理する）。`alg` が存在すれば `RS256`。
- `x5c` が存在する場合は [GOJ-X5C](#goj-x5c-x5c-と証明書の整合) の証明書との整合条件も満たす。`x5c` 自体は必須ではない。

法の素因数分解など、公開鍵だけでは確認できない性質まで保証するものではありません。採用した公開鍵は、最後に `RS256` の実際の署名検証に使用します。公開鍵の構築に失敗する場合は署名不一致と読み替えず、除外した JWK として扱います。

## GOJ-X5C `x5c` と証明書の整合

**標準仕様**：RFC 7517 §4.7 は、`x5c` を Base64（Base64URL ではない）で表した X.509 証明書の配列とし、先頭証明書の公開鍵が JWK の公開鍵パラメーターと一致することを **MUST** とします。`use`、`alg` などの関連メンバーも、証明書に記載された対応情報と意味的に整合することが **MUST** です（[RFC 7517 §4.6–§4.7](https://www.rfc-editor.org/rfc/rfc7517.html#section-4.7)）。

**GATES の受理条件**：`x5c` があれば、1件以上の文字列を含む配列として読み、各要素が Base64 DER の X.509 証明書として解析できることを要求します。先頭証明書には RSA 公開鍵を要求し、その `n` / `e` を JWK と一致させます。2件目以降がある場合は、後続の証明書が直前の証明書の発行者であることを署名関係で確認します。証明書に Key Usage がある場合は、署名用途と矛盾しない `digitalSignature` を要求します。`alg=RS256` は証明書の公開鍵アルゴリズム・用途と整合させ、**証明書自身の発行者による署名アルゴリズムが `RS256` と同じであることは要求しません**。これらに不一致があれば鍵を除外します。RFC 7517 §4.6 は `alg` が「証明書に指定されたアルゴリズム」と対応することを求めますが、どのフィールドかを定めていません。GATES はこれを公開鍵のアルゴリズムと解釈します。発行者の署名アルゴリズムは発行者が証明書に署名した方式であり、この鍵で JWT を検証する方式ではないためです。

`x5c` の証明書を新たな信頼アンカーにはせず、PKIX の認証パス全体の信頼性やルート証明書の信頼性を判定する方式も採りません。署名検証鍵は引き続き固定 Issuer の JWKS によって信頼します。

## GOJ-JWKS JWK Set の拒否とエラー分類

**秘密情報を含む集合**：JWKS 内のいずれかの JWK が `kty=oct`、または `kty` を問わず秘密鍵・対称鍵の値のメンバー（対称鍵の `k`、秘密鍵の `d`、`p`、`q`、`dp`、`dq`、`qi`、`oth`、AKP 鍵の秘密鍵の `priv`（[RFC 9964 §3](https://www.rfc-editor.org/rfc/rfc9964.html#section-3)））を含む場合は、名前の重複などでその JWK を JWK として読めなくても、またほかに正常な鍵があっても、JWKS 全体を拒否し、内部障害とします。該当する値はログ・エラー応答に出しません。未知の公開情報メンバーは、存在するだけでは拒否しません。

**集合・鍵・JWT の失敗分類**：取得・キャッシュ・再取得の手順はコンポーネント設計が担当します。最終的に検証対象となる JWKS の分類は次のとおりです。

| 条件 | GATES の扱い・最終エラー |
|---|---|
| JWKS の JSON 不正、`keys` が配列でない、集合ルートに名前の重複がある | 集合全体を拒否。内部障害 |
| 秘密鍵・対称鍵の値が含まれる | 集合全体を拒否。内部障害 |
| 個別 JWK の構文・型・公開鍵構築・`x5c` 整合の失敗 | その鍵を除外。残りの使用可能な鍵で判定を続ける |
| 除外後に使用可能な鍵が0件 | 検証基盤の異常。内部障害 |
| JWT の `kid` に一致する使用可能な鍵がなく（`kid` が元の JWKS に存在しない、または除外した JWK にだけ一致）、ほかに使用可能な鍵がある | 再取得規則を経て認証失敗 |
| JWT の `kid` が使用可能な鍵に複数一致 | 一意に選べないため、再取得規則を経て認証失敗。総当たり検証しない |
| JWT の `kid` が使用可能な鍵に1件一致したが署名が不一致 | 再取得規則を経て認証失敗 |
| Discovery / JWKS の通信失敗・HTTP エラー・読み取り失敗 | JWT 自体の不正とは区別し、コンポーネント設計の外部障害分類に従う |

同じ `kid` の除外した JWK と使用可能な鍵が混在しても、使用可能な鍵が1件だけならその鍵を選びます。除外した JWK は、JWT の判定とは別に、運用者が Issuer の鍵情報の不整合に気付けるよう記録します。記録の方法はコンポーネント設計で定めます。

**観測と限界**：取得した JWKS は4件とも2048 bit RSA 公開鍵で、`x5c` / `x5t` は一部にのみ存在し、`kid` の形式も混在しています（[GitHub OIDC JWKS](../research/github/oidc-jwks.md)）。3件の `x5c` の証明書公開鍵が `n` / `e` と一致することは調査記録を用いて検証済みです。この観測を将来の JWKS の保証とはみなしません。

## Traceability

| GATES の受理条件 | 要求・設計上の根拠 |
|---|---|
| 署名と検証鍵 | [Requirements FR-002、FR-003](../spec/requirements.md#51-authentication) |
| 署名鍵の固定 Issuer からの取得 | [JWT Constraints §6](../spec/jwt-constraints.md#6-jwt-security-considerations)、[Requirements FR-002](../spec/requirements.md#51-authentication) |

## Verification Cases

| 対象 | 受理例・拒否例の焦点 | 期待結果 |
|---|---|---|
| JSON の曖昧さ | Header の重複名、Unicode エスケープによる重複、不正 UTF-8 | 署名が正しくても拒否 |
| JOSE Header | 未知 Header、`crit`、`b64`、`cty=JWT` / `cty=jwt` / その他の `cty`、`alg=none`、`kid` 欠落、改行入り JWT | 未知 Header は無視し、`cty` を含む未対応の形式・拡張を拒否 |
| JWKS | `kid` 不在、除外した JWK だけが一致、複数一致、公開指数の境界、`x5c`・Key Usage の不整合、鍵ゼロ、秘密情報混入、取得障害 | GOJ-JWKS のエラー分類に一致し、すべて Fail Closed |

## Outstanding Verification

ID の `JV-S-` は、本書の未確認事項を表します。

| ID | 未確認事項 | 確認方法 |
|---|---|---|
| JV-S-01 | 複数証明書を含む `x5c` の実測がない | 証明書間の署名関係を含むテストで受理条件を確認 |

## Sources

- [JWT Constraints](../spec/jwt-constraints.md) — 規範と適用範囲の整理
- [GitHub OIDC Discovery](../research/github/oidc-discovery.md) — Discovery の公開値
- [GitHub OIDC JWKS](../research/github/oidc-jwks.md) — 鍵情報
- [RFC 7515 — JSON Web Signature](https://www.rfc-editor.org/rfc/rfc7515.html)
- [RFC 7517 — JSON Web Key](https://www.rfc-editor.org/rfc/rfc7517.html)
- [RFC 7518 — JSON Web Algorithms](https://www.rfc-editor.org/rfc/rfc7518.html)
- [RFC 7519 — JSON Web Token](https://www.rfc-editor.org/rfc/rfc7519.html)
- [RFC 7797 — JWS Unencoded Payload Option](https://www.rfc-editor.org/rfc/rfc7797.html)
- [RFC 8725 — JSON Web Token Best Current Practices](https://www.rfc-editor.org/rfc/rfc8725.html)
- [RFC 9964 — ML-DSA for JSON Object Signing and Encryption (JOSE) and CBOR Object Signing and Encryption (COSE)](https://www.rfc-editor.org/rfc/rfc9964.html)
- [OpenID Connect Core 1.0](https://openid.net/specs/openid-connect-core-1_0.html)
- [OpenID Connect Discovery 1.0](https://openid.net/specs/openid-connect-discovery-1_0.html)
