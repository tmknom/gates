# GATES OIDC JWT Signature

本ドキュメントは、GATES 用 OIDC JWT の直列化、JOSE Header、署名の検証鍵に GATES が課す受理条件を定めます。
Claims Set の受理条件は [GATES OIDC JWT Claims](gates-oidc-jwt-claims.md) に定め、規則の ID、関係の表の読み方、受理の分類、GATES の役割に当たる Actor、GATES 用 OIDC JWT を ID Token として扱わないことも、そちらに従います。
各規則の ID は同じ `GOJ-` の接頭辞で始まります。

Discovery / JWKS の取得時刻、キャッシュの存続、再取得の上限、タイムアウト、応答サイズ、失敗時の戻り値は本書の受理条件ではなく、OIDC Verifier のコンポーネント設計が扱います。

## Serialization

### GOJ-SERIALIZATION-COMPACT JWS Compact Serialization

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7515-BASE64URL-FORMAT | なし |
| Satisfies | RFC7515-COMPACT-FORMAT | なし |
| Satisfies | RFC7519-SERIALIZATION | なし |
| Satisfies | RFC7519-VALIDATE-REJECT | RFC 7519 §7.2 の手順のうち、`.` で区切った各部分を改行や空白を含まずに Base64url デコードできることの確認に当たる |
| Narrows | RFC7519-SERIALIZATION | JWE Compact Serialization を拒否する。GATES 用 OIDC JWT は署名した JWT で、GATES は復号の鍵を持たないため |
| Accepts | GH-OIDC-HEADER-OBSERVED | なし |

JWS Compact Serialization だけを受理し、3部分以外の形、JSON Serialization、Base64URL 以外の文字、余分な padding・空白は拒否します。

JWT の生の文字列に CR / LF を含む場合も拒否します。JWS Compact Serialization の符号化された構成要素に改行を含めることはできず、検証ライブラリが読み飛ばす挙動を受理条件の代用にしません。

## JOSE Header

### GOJ-HEADER-JSON JOSE Header の JSON

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7515-HEADER-NAME-DUPLICATE | 重複した Header Parameter 名を持つ JWS を拒否する側を選ぶ |
| Satisfies | RFC7515-HEADER-NAME-UNIQUE | なし |
| Satisfies | RFC7515-JSON-STRICT | なし |
| Satisfies | RFC7515-JSON-TRAILING-INVALID | なし |
| Satisfies | RFC7519-VALIDATE-REJECT | RFC 7519 §7.2 の手順のうち、JOSE Header が UTF-8 で表した完全に有効な JSON オブジェクトであることの確認に当たる |
| Satisfies | RFC8725-UTF8-APPLICATION | なし |
| Satisfies | RFC8725-UTF8-IMPLEMENTATION | なし |
| Accepts | GH-OIDC-HEADER-OBSERVED | なし |

JOSE Header で、エスケープ解除後に同一となる JSON 名の重複や不正な UTF-8 を拒否します。

### GOJ-HEADER-ALG `alg`

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7515-ALG-APPLICATION-ACCEPTABLE | なし |
| Satisfies | RFC7515-ALG-REQUIRED | なし |
| Satisfies | RFC7515-ALG-UNDERSTAND | なし |
| Satisfies | RFC7515-STRING-COMPARE | なし |
| Satisfies | RFC7518-NONE-ACCEPT-DEFAULT | なし |
| Satisfies | RFC7518-NONE-GLOBAL | なし |
| Satisfies | RFC7519-ALG-ACCEPTABLE | なし |
| Satisfies | RFC8725-ALG-AGILITY | 受理するアルゴリズムは GOJ-HEADER-ALG だけが定め、その改訂で入れ替える |
| Satisfies | RFC8725-ALG-CURRENT | なし |
| Satisfies | RFC8725-ALG-NONE-CONSUME | なし |
| Satisfies | RFC8725-ALG-SET-RESTRICT | なし |
| Satisfies | RFC8725-ALG-SET-SPECIFY | GATES は、受理するアルゴリズムを `RS256` だけとしてライブラリに指定する |
| Narrows | RFC7515-ALG-FORMAT | `RS256` のほかの値を拒否する。GitHub が公開する署名のアルゴリズムが `RS256` だけで、ほかのアルゴリズムを受理すると、アルゴリズムの取り違えを使った攻撃の余地が残るため（FR-AUTHN-007） |
| Accepts | GH-OIDC-DISCOVERY-ALG | なし |
| Accepts | GH-OIDC-HEADER-EXAMPLE | なし |
| Accepts | GH-OIDC-HEADER-OBSERVED | なし |

必須。`RS256` のみ。`none`、`PS256` 等は拒否します。

### GOJ-HEADER-KID `kid`

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7515-KID-FORMAT | なし |
| Narrows | RFC7515-KID-FORMAT | 空文字列の `kid` を拒否する。空の値では JWKS の鍵を選べないため |
| Narrows | RFC7515-KID-OPTIONAL | `kid` がない JWT を拒否する。GitHub の JWKS は複数の鍵を持ち、鍵を総当たりで試さずに一つを選ぶため |
| Accepts | GH-OIDC-HEADER-DOCUMENTED | 公式資料は `kid` の値と型を書かないため、空でない文字列であることは GH-OIDC-HEADER-KID-JWKS と GH-OIDC-JWKS-KID-FORMAT に依存する |
| Accepts | GH-OIDC-HEADER-EXAMPLE | なし |
| Accepts | GH-OIDC-HEADER-KID-JWKS | Header の `kid` は JWKS の鍵の `kid` に一致する |
| Accepts | GH-OIDC-HEADER-OBSERVED | なし |
| Accepts | GH-OIDC-JWKS-KID-FORMAT | JWKS の鍵の `kid` は、どれも空でない文字列である |

必須。空でない JSON 文字列とし、JWKS 内の使用可能な鍵を選択するために利用します（[GOJ-JWKS-KID](#goj-jwks-kid-kid-による鍵の選択)）。値は大文字小文字を区別します。`kid` は外部 URL やファイルパスとして解釈せず、固定した Issuer の JWKS 内で鍵を引くためだけに使います。`kid` を特定の UUID / 16進数字形式に限定しません（GH-OIDC-JWKS-KID-FORMAT）。

### GOJ-HEADER-TYP `typ`

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7515-HEADER-DEFINED-UNKNOWN-IGNORE | `typ` の値を使わない |
| Accepts | GH-OIDC-HEADER-DOCUMENTED | なし |
| Accepts | GH-OIDC-HEADER-EXAMPLE | なし |
| Accepts | GH-OIDC-HEADER-OBSERVED | なし |

存在を必須にしません。値を受理の判定と署名鍵の取得先に使いません。

### GOJ-HEADER-CTY `cty`

| Relation | Clause | Note |
|---|---|---|
| Narrows | RFC7515-CTY-OPTIONAL | `cty` を持つ JWT を、値を問わず拒否する。GATES は Nested JWT に対応せず、Claims Set のほかの内容を受理しないため |
| Accepts | GH-OIDC-HEADER-OBSERVED | なし |
| Possibly Rejects | GH-OIDC-HEADER-PARAMETERS | `cty` を持つ JWT を拒否する。GitHub が `cty` を加えても、GATES が対応しない Nested JWT を受理しないことを優先する |

`cty` が存在する JWT は、値を問わず拒否します。`cty` を一律に拒否するのは、Nested JWT に対応しない GATES の JWS プロファイルの追加制約です。GATES は `cty` の意味を理解したうえで拒否するため、理解しない Header Parameter の無視（RFC7515-HEADER-DEFINED-UNKNOWN-IGNORE）の対象ではありません。

### GOJ-HEADER-CRIT `crit`

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7515-CRIT-UNDERSTAND | なし |
| Satisfies | RFC7515-CRIT-UNSUPPORTED-INVALID | なし |
| Narrows | RFC7515-CRIT-OPTIONAL | `crit` を持つ JWT を、値を問わず拒否する。GATES は拡張を実装せず、未対応の拡張が意味する処理を無視しないため（FR-AUTHN-007） |
| Accepts | GH-OIDC-HEADER-OBSERVED | なし |
| Possibly Rejects | GH-OIDC-HEADER-PARAMETERS | `crit` を持つ JWT を拒否する。GitHub が拡張を使っても、GATES が実装しない拡張の処理を無視しないことを優先する |

存在すれば拒否します。`crit` が空・不正であっても受理しません。

### GOJ-HEADER-B64 `b64`

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7797-JWT-B64-FALSE | なし |
| Narrows | RFC7797-B64-OPTIONAL | `true` の `b64` も拒否する。GATES が扱う JWT は Payload を Base64url で符号化した形に限り、`b64` の扱いをライブラリに任せないため（FR-AUTHN-007） |
| Accepts | GH-OIDC-HEADER-OBSERVED | なし |
| Possibly Rejects | GH-OIDC-HEADER-PARAMETERS | `b64` を持つ JWT を拒否する。`true` の `b64` は省いた場合と同じ意味だが、受理する Header の形を限ることを優先する |

`true` / `false` とも受理しません。`b64` は RFC 7797 が定める Header Parameter で、GATES は意味を理解したうえで拒否するため、理解しない Header Parameter の無視（RFC7515-HEADER-EXTENSION-UNKNOWN-IGNORE）の対象ではありません。

### GOJ-HEADER-KEY-REFERENCE 鍵を参照・運ぶ Header Parameter

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7515-HEADER-DEFINED-UNKNOWN-IGNORE | `jku`、`jwk`、`x5u`、`x5c`、`x5t`、`x5t#S256` の値を使わない |
| Satisfies | RFC8725-JKU-X5U-SSRF | `jku` と `x5u` の URL から取得しない |
| Accepts | GH-OIDC-HEADER-EXAMPLE | なし |
| Accepts | GH-OIDC-HEADER-OBSERVED | なし |
| Possibly Rejects | GH-OIDC-HEADER-PARAMETERS | JWK として読めない `jwk` と、X.509 証明書の配列として読めない `x5c` を持つ JWT を拒否する。検証ライブラリが値を解析するためで、読める値なら存在だけでは拒否しない |

`jku`、`jwk`、`x5u`、`x5c`、`x5t`、`x5t#S256` を、鍵の取得先・検証鍵選択の根拠にしません。存在だけを拒否理由にしませんが、`jwk` は JWK として、`x5c` は X.509 証明書の配列として読めなければ拒否します。

### GOJ-HEADER-UNKNOWN 理解しない Header Parameter

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7515-HEADER-EXTENSION-UNKNOWN-IGNORE | なし |
| Possibly Rejects | GH-OIDC-HEADER-PARAMETERS | JSON の文字列でない `nonce` を持つ JWT を拒否する。検証ライブラリが `nonce` を文字列として解析するためで、ほかの未知の Header Parameter は拒否しない |

無視します。未知であることだけでは拒否しません。ただし `nonce` は、JSON の文字列でなければ拒否します。

## Discovery

GitHub の Discovery の Metadata は、`authorization_endpoint` と `token_endpoint` を持ちません（GH-OIDC-DISCOVERY-MEMBERS）。
OpenID Provider には OIDCD-AUTHORIZATION-ENDPOINT-REQUIRED と OIDCD-TOKEN-ENDPOINT-REQUIRED が課され、GitHub の Metadata は OpenID Provider Metadata の要求を満たしません。
ただし、Metadata の不足だけで個々の JWT の署名・Claim が不正だと断定することはできません。
GATES は、認証 Endpoint、`client_id`、`nonce` を前提にした OpenID Connect の認証フローを実行せず、Metadata のうち `issuer` と `jwks_uri` だけを使います。
完全な Discovery クライアントとして標準準拠しているとは主張しません。

### GOJ-DISCOVERY-ISSUER Discovery の取得と `issuer`

| Relation | Clause | Note |
|---|---|---|
| Satisfies | OIDCD-CONFIG-GET | なし |
| Satisfies | OIDCD-CONFIG-ISSUER-MATCH | なし |
| Satisfies | OIDCD-CONFIG-TLS-CHECK | なし |
| Satisfies | OIDCD-CONFIG-VALIDATION-ABORT | なし |
| Satisfies | OIDCD-CONFIG-VALIDATION-UNUSED | なし |
| Satisfies | OIDCD-STRING-CODE-POINT | なし |
| Satisfies | OIDCD-STRING-NO-NORMALIZATION | なし |
| Satisfies | OIDCD-STRING-UNESCAPE | なし |
| Satisfies | OIDCD-TLS-CERTIFICATE | なし |
| Satisfies | OIDCD-TLS-CONFIDENTIALITY | なし |
| Satisfies | OIDCD-TLS-SUPPORT | なし |
| Accepts | GH-OIDC-DISCOVERY-ISSUER | なし |
| Accepts | GH-OIDC-DISCOVERY-LOCATION | なし |
| Accepts | GH-OIDC-ISS | なし |

署名検証鍵は、[GATES OIDC JWT Claims](gates-oidc-jwt-claims.md) の GOJ-ISS の固定 Issuer の検証済み Discovery / JWKS だけから得ます。
固定 Issuer の Discovery を、サーバー証明書を検証する HTTPS で、リダイレクトに従わずに取得し、**一致を確認した `issuer`** だけを使用します。
Discovery に異なる `issuer` があればその Metadata を利用しません。

### GOJ-DISCOVERY-JWKS-URI 鍵の取得先

| Relation | Clause | Note |
|---|---|---|
| Satisfies | OIDCD-JWKS-URI-HTTPS | なし |
| Satisfies | OIDCD-TLS-CERTIFICATE | なし |
| Satisfies | OIDCD-TLS-CONFIDENTIALITY | なし |
| Satisfies | OIDCD-TLS-SUPPORT | なし |
| Satisfies | RFC8725-ISS-KEY-VALIDATE | GOJ-ISS とともに満たす。`iss` を固定の Issuer に限り、鍵をその Issuer の `jwks_uri` だけから得る |
| Accepts | GH-OIDC-DISCOVERY-JWKS-URI | なし |

一致を確認した `issuer` を持つ Discovery の JWKS の取得先 `jwks_uri` だけを使用します。
JWKS の URL は Issuer と同じ HTTPS のホスト・ポートに制限し、リダイレクトに従いません。
JWT の Header やリクエストが任意の鍵取得 URL を指定することも許しません。

## JWK

JWK ごとの受理条件です。満たさない JWK の扱いは [GOJ-JWKS-USABLE](#goj-jwks-usable-使用可能な鍵) に定めます。

### GOJ-JWK-JSON JWK の JSON

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7517-MEMBER-NAME-DUPLICATE | 重複したメンバー名を持つ JWK を受理しない側を選ぶ |
| Satisfies | RFC7517-MEMBER-NAME-UNIQUE | なし |
| Satisfies | RFC7517-MEMBER-UNKNOWN-IGNORE | なし |
| Accepts | GH-OIDC-JWKS-RSA-MEMBERS | なし |

JWK は、名前の重複のない JSON オブジェクトとします。未知の公開情報メンバーは、存在するだけでは拒否しません。

### GOJ-JWK-RSA RSA 公開鍵

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7518-BASE64URLUINT-FORMAT | なし |
| Satisfies | RFC7518-BASE64URLUINT-MINIMAL | なし |
| Satisfies | RFC7518-RS-KEY-SIZE | なし |
| Satisfies | RFC7518-RSA-E-FORMAT | なし |
| Satisfies | RFC7518-RSA-KTY | なし |
| Satisfies | RFC7518-RSA-N-FORMAT | なし |
| Satisfies | RFC7518-RSA-PUBLIC-MEMBERS | なし |
| Narrows | RFC7517-KTY-FORMAT | `RSA` のほかの `kty` の JWK を受理しない。`RS256` の検証に使える鍵は RSA 公開鍵だけのため |
| Narrows | RFC7518-RSA-E-FORMAT | 偶数か 3 未満か `2^31 − 1` を超える `e` の JWK を受理しない。RSA 公開鍵として妥当でない値と、署名の検証で拒否される値を、署名の不一致に見せないため |
| Narrows | RFC7518-RSA-N-FORMAT | 偶数の `n` の JWK を受理しない。RSA 公開鍵として妥当でない値を、署名の不一致に見せないため |
| Accepts | GH-OIDC-JWKS-MODULUS | なし |
| Accepts | GH-OIDC-JWKS-RSA-MEMBERS | なし |

- `kty=RSA`。`n` と `e` は正しい Base64urlUInt として復号できる。
- RSA の法 `n` は2048 bit 以上の正の奇数、公開指数 `e` は3以上の奇数。`e` は `2^31 − 1` 以下とする。

法の素因数分解など、公開鍵だけでは確認できない性質まで保証するものではありません。公開鍵の構築に失敗する場合は署名不一致と読み替えず、受理条件を満たさない JWK として扱います。

### GOJ-JWK-KID JWK の `kid`

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7517-KID-FORMAT | なし |
| Narrows | RFC7517-KID-FORMAT | 空文字列の `kid` の JWK を受理しない。JWT の `kid` で選べないため |
| Narrows | RFC7517-KID-OPTIONAL | `kid` がない JWK を受理しない。JWT の `kid` で選べないため |
| Accepts | GH-OIDC-JWKS-KID-FORMAT | `kid` の形式を限らない |
| Accepts | GH-OIDC-JWKS-RSA-MEMBERS | なし |

`kid` は空でない JSON 文字列とします。

### GOJ-JWK-USAGE `use` と `key_ops`

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7517-KEY-OPS-DUPLICATE | なし |
| Satisfies | RFC7517-USE-KEY-OPS-CONSISTENT | なし |
| Narrows | RFC7517-KEY-OPS-FORMAT | `verify` を含まない `key_ops` の JWK と、`use` もある場合に `sign` と `verify` のほかの値を含む `key_ops` の JWK を受理しない。署名の検証に使えると表した鍵だけを使うため |
| Narrows | RFC7517-USE-FORMAT | `sig` のほかの `use` の JWK を受理しない。署名の検証に使えると表した鍵だけを使うため |
| Accepts | GH-OIDC-JWKS-RSA-MEMBERS | なし |

`use` が存在すれば `sig`、`key_ops` が存在すれば文字列の配列で `verify` を含み、重複がない。`use` と `key_ops` が両方存在する場合は、`use=sig` と整合させるため、`key_ops` の値を `sign` と `verify` に限る（`["verify"]` と `["sign", "verify"]` を受理する）。

### GOJ-JWK-ALG JWK の `alg`

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC8725-KEY-ALG-SINGLE | なし |
| Narrows | RFC7517-ALG-FORMAT | `RS256` のほかの `alg` の JWK を受理しない。鍵を GOJ-HEADER-ALG の `RS256` だけで使うため |
| Accepts | GH-OIDC-JWKS-RSA-MEMBERS | なし |

`alg` が存在すれば `RS256` とします。

### GOJ-X5C-KEY 鍵の証明書のメンバー

| Relation | Clause | Note |
|---|---|---|
| Satisfies | OIDCD-JWKS-X5C-BARE-KEY-MATCH | なし |
| Satisfies | OIDCD-JWKS-X5C-BARE-KEY-PRESENT | `n` と `e` は GOJ-JWK-RSA が求める |
| Accepts | GH-OIDC-JWKS-X5C | なし |
| Accepts | GH-OIDC-JWKS-X5C-KEY | なし |

RFC 7517 §4.7 は、`x5c` を Base64（Base64URL ではない）で表した X.509 証明書の配列とし、先頭証明書の公開鍵が JWK の公開鍵パラメーターと一致することを **MUST** とします。`use`、`alg` などの関連メンバーも、証明書に記載された対応情報と意味的に整合することが **MUST** です（[RFC 7517 §4.6–§4.7](https://www.rfc-editor.org/rfc/rfc7517.html#section-4.7)）。§4.8–§4.9 は、`x5t` と `x5t#S256` を証明書の指紋とし、`x5u` を証明書の URI とします。これらの規定は [RFC 7517 JSON Web Key](../external/standards/rfc7517.md) の条項の対象外のため、関係の表に挙げていません。

`x5c` があれば、1件以上の文字列を含む配列として読み、各要素が Base64 DER の X.509 証明書として解析できることを要求します。先頭証明書には RSA 公開鍵を要求し、その `n` / `e` を JWK と一致させます。証明書に Key Usage がある場合は、署名用途と矛盾しない `digitalSignature` を要求します。`alg=RS256` は証明書の公開鍵アルゴリズム・用途と整合させ、**証明書自身の発行者による署名アルゴリズムが `RS256` と同じであることは要求しません**。これらに不一致があれば、その JWK は受理条件を満たしません。RFC 7517 §4.6 は `alg` が「証明書に指定されたアルゴリズム」と対応することを求めますが、どのフィールドかを定めていません。GATES はこれを公開鍵のアルゴリズムと解釈します。発行者の署名アルゴリズムは発行者が証明書に署名した方式であり、この鍵で JWT を検証する方式ではないためです。`x5c` 自体は必須ではありません。

`x5t` と `x5t#S256` があれば、Base64url で正しく符号化された SHA-1 と SHA-256 の指紋の長さであることを要求し、`x5c` もあれば先頭証明書の指紋と一致させます。`x5u` があれば、URL として解析できることを要求します。`x5u` の URL から証明書を取得しません。これらに不一致があれば、その JWK は受理条件を満たしません。

`x5c` の証明書を新たな信頼アンカーにはせず、PKIX の認証パス全体の信頼性やルート証明書の信頼性を判定する方式も採りません。署名検証鍵は引き続き固定 Issuer の JWKS によって信頼します。

### GOJ-X5C-CHAIN `x5c` の証明書の関係

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-JWKS-X5C | 証明書が 1 個の `x5c` には、確かめる関係がない |
| Possibly Rejects | GH-OIDC-JWKS-X5C-CHAIN | 2 個目以降の証明書が直前の証明書に署名していない `x5c` を持つ JWK を受理しない。RFC 7517 §4.7 が後続の証明書に求める関係であり、その鍵で署名した JWT は、ほかに一致する鍵がなければ認証失敗になる |

`x5c` に2件目以降の証明書がある場合は、後続の証明書が直前の証明書の発行者であることを署名関係で確認します。不一致があれば、その JWK は受理条件を満たしません。

## JWK Set

取得・キャッシュ・再取得の手順はコンポーネント設計が担当します。Discovery / JWKS の通信失敗・HTTP エラー・読み取り失敗は、JWT 自体の不正とは区別し、コンポーネント設計の外部障害分類に従います。

### GOJ-JWKS-FORMAT JWK Set の構造

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7517-JWKS-KEYS-REQUIRED | なし |
| Satisfies | RFC7517-JWKS-MEMBER-NAME-DUPLICATE | 重複したメンバー名を持つ JWK Set を受理しない側を選ぶ |
| Satisfies | RFC7517-JWKS-MEMBER-NAME-UNIQUE | なし |
| Satisfies | RFC7517-JWKS-MEMBER-UNKNOWN-IGNORE | なし |
| Accepts | GH-OIDC-JWKS-RSA-MEMBERS | なし |

JWKS の JSON 不正、`keys` が配列でない、集合ルートに名前の重複がある場合は、集合全体を拒否し、内部障害とします。`keys` のほかのメンバーは無視します。

### GOJ-JWKS-SECRET 秘密情報を含む JWK Set

| Relation | Clause | Note |
|---|---|---|
| Satisfies | OIDCD-JWKS-NO-PRIVATE | なし |
| Satisfies | RFC9964-AKP-PRIV-PUBLIC-KEY | なし |
| Accepts | GH-OIDC-JWKS-RSA-MEMBERS | なし |

JWKS 内のいずれかの JWK が `kty=oct`、または `kty` を問わず秘密鍵・対称鍵の値のメンバー（対称鍵の `k`、秘密鍵の `d`、`p`、`q`、`dp`、`dq`、`qi`、`oth`、AKP 鍵の秘密鍵の `priv`）を含む場合は、名前の重複などでその JWK を JWK として読めなくても、またほかに正常な鍵があっても、JWKS 全体を拒否し、内部障害とします。該当する値はログ・エラー応答に出しません。
どのメンバーが秘密の情報かは、RFC7518-EC-PRIVATE-D、RFC7518-RSA-PRIVATE-MEMBERS、RFC7518-OCT-KEY-VALUE、RFC9964-AKP-PRIV-PUBLIC-KEY に従います。

秘密鍵・対称鍵の値を含む JWKS は OpenID Provider が作ってはならないもので（OIDCD-JWKS-NO-PRIVATE）、その JWK だけを除外せず集合全体を拒否するのは GATES の追加制約です。

### GOJ-JWKS-USABLE 使用可能な鍵

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7517-JWKS-KEY-IGNORE | なし |
| Accepts | GH-OIDC-JWKS-MODULUS | なし |
| Accepts | GH-OIDC-JWKS-RSA-MEMBERS | なし |

JWK の受理条件（GOJ-JWK-JSON、GOJ-JWK-RSA、GOJ-JWK-KID、GOJ-JWK-USAGE、GOJ-JWK-ALG、GOJ-X5C-KEY、GOJ-X5C-CHAIN）をすべて満たし、RSA 公開鍵として検証処理に渡せる JWK を **使用可能な鍵** とします。これは署名の一致を意味しません。受理条件を満たさない JWK は **除外した JWK** とし、保持しません。

| 条件 | GATES の扱い・最終エラー |
|---|---|
| 個別 JWK の構文・型・公開鍵構築・`x5c` 整合の失敗 | その鍵を除外。残りの使用可能な鍵で判定を続ける |
| 除外後に使用可能な鍵が0件 | 検証基盤の異常。内部障害 |

除外した JWK は、JWT の判定とは別に、運用者が Issuer の鍵情報の不整合に気付けるよう記録します。記録の方法はコンポーネント設計で定めます。

### GOJ-JWKS-KID `kid` による鍵の選択

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7515-KID-JWK-MATCH | なし |
| Satisfies | RFC7515-STRING-COMPARE | なし |
| Satisfies | RFC7517-KID-HEADER-MATCH | なし |
| Satisfies | RFC7517-STRING-COMPARE | なし |
| Narrows | RFC7517-KID-DISTINCT | JWT の `kid` が使用可能な鍵の複数に一致すれば拒否する。鍵を総当たりで試さないため |
| Accepts | GH-OIDC-HEADER-KID-JWKS | なし |

| 条件 | GATES の扱い・最終エラー |
|---|---|
| JWT の `kid` に一致する使用可能な鍵がなく（`kid` が元の JWKS に存在しない、または除外した JWK にだけ一致）、ほかに使用可能な鍵がある | 再取得規則を経て認証失敗 |
| JWT の `kid` が使用可能な鍵に複数一致 | 一意に選べないため、再取得規則を経て認証失敗。総当たり検証しない |

同じ `kid` の除外した JWK と使用可能な鍵が混在しても、使用可能な鍵が1件だけならその鍵を選びます。

### GOJ-SIGNATURE 署名の検証

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7515-SIGNATURE-VALID-REQUIRED | なし |
| Satisfies | RFC7515-SIGNATURE-VALIDATE-ALG | なし |
| Satisfies | RFC7518-RS256-VALIDATE | なし |
| Satisfies | RFC7519-VALIDATE-REJECT | RFC 7519 §7.2 の手順のうち、JWS の検証に当たる |
| Satisfies | RFC8725-ALG-HEADER-MATCH | なし |
| Satisfies | RFC8725-CRYPTO-VALIDATE | なし |
| Satisfies | RFC8725-KEY-ALG-CHECK | なし |

選んだ使用可能な鍵を、最後に `RS256` の実際の署名検証に使用します。JWT の `kid` が使用可能な鍵に1件一致したが署名が不一致の場合は、再取得規則を経て認証失敗とします。

## Standard Clauses Not Applied

GATES の役割に当たる強い規範の条項のうち、直列化、JOSE Header、Discovery、JWK を扱い、GATES に適用しない条項です。
GATES の役割に当たる Actor と、Claims Set を扱う条項は [GATES OIDC JWT Claims](gates-oidc-jwt-claims.md) に挙げます。

| Clause | Reason |
|---|---|
| OIDCC-CODE-ISSUER-KEYS | Authorization Code Flow の Token Response に含まれる ID Token に限る。GATES は Issuer の鍵だけを使う（GOJ-DISCOVERY-JWKS-URI） |
| OIDCC-CODE-SIGNATURE-VALIDATE | Authorization Code Flow の ID Token に限る。GATES は GOJ-SIGNATURE で署名を検証する |
| OIDCC-IMPLICIT-SIGNATURE | Implicit Flow の ID Token に限る |
| OIDCC-SIGNING-MAC-PUBLIC-CLIENT | 公開 Client の Relying Party に限る。GATES は OpenID Provider に登録した Client ではなく、MAC による署名を受理しない（GOJ-HEADER-ALG） |
| OIDCD-CONFIG-ISSUER-ID-TOKEN | ID Token に限る。GATES は `iss`（GOJ-ISS）と Discovery の `issuer`（GOJ-DISCOVERY-ISSUER）を、同じ固定の Issuer と照合する |
| OIDCD-CONFIG-PATH-SLASH | パスを持つ Issuer に限る。固定の Issuer（GH-OIDC-ISS）はパスを持たない |
| RFC7515-ALG-SIGNATURE-MATCH | 署名の値に含まれるアルゴリズムの情報を使って検証するライブラリに限る。GATES の署名の検証が使う Go の `crypto/rsa` は、`alg` から決めたハッシュ関数で期待する値を組み立てて比べ、署名の値の中のハッシュ関数の情報を使わない |
| RFC7515-CTY-PREFIX-RESTORE | `cty` の値を使う Recipient に限る。GATES は `cty` の値を使わずに拒否する（GOJ-HEADER-CTY） |
| RFC7515-KEY-HEADER-PROTECT | 鍵そのもののほかに、鍵の Header Parameter の情報を信頼の判断に使う場合に限る。GATES は `kid` を鍵の選択だけに使い（GOJ-HEADER-KID）、ほかの鍵の Header Parameter を使わない（GOJ-HEADER-KEY-REFERENCE） |
| RFC7515-TYP-PREFIX-RESTORE | `typ` の値を使う Recipient に限る。GATES は `typ` の値を使わない（GOJ-HEADER-TYP） |
| RFC7515-UNICODE-NON-BMP-REJECT | JSON の実装が基本多言語面の外の文字を保持して比較できない場合に限る。GATES が使う Go の JSON の実装は、これらの文字を UTF-8 で保持して比較する |
| RFC7518-NONE-ACCEPT-SPECIFIED | Unsecured JWS に対応する実装に限る。GATES は対応しない（GOJ-HEADER-ALG） |
| RFC7518-NONE-RECIPIENT-VERIFY | `alg` が `none` の JWS に限る。GATES は `none` を拒否する（GOJ-HEADER-ALG） |
| RFC7518-PS-KEY-SIZE | RSASSA-PSS のアルゴリズムに限る。GATES は `RS256` だけを受理する（GOJ-HEADER-ALG） |
| RFC8725-CRYPTO-NESTED-VALIDATE | Nested JWT に限る。GATES は Nested JWT を受理しない（GOJ-HEADER-CTY） |

## Traceability

| GATES の受理条件 | 要求・設計上の根拠 |
|---|---|
| 署名と検証鍵 | [Requirements](../spec/requirements.md) の FR-AUTHN-002、FR-AUTHN-003 |
| 署名鍵の固定 Issuer からの取得 | [Requirements](../spec/requirements.md) の FR-AUTHN-002 |
| `crit`、`b64` の拒否と、JWK の用途、署名方式、強度の条件 | [Requirements](../spec/requirements.md) の FR-AUTHN-007 |
| JWKS の取得の障害と使用可能な鍵がない JWKS を、認証失敗と区別すること | [Requirements](../spec/requirements.md) の FR-OPS-002 |

## Verification Cases

| 対象 | 受理例・拒否例の焦点 | 期待結果 |
|---|---|---|
| JSON の曖昧さ | Header の重複名、Unicode エスケープによる重複、不正 UTF-8 | 署名が正しくても拒否 |
| JOSE Header | 未知 Header、`crit`、`b64`、`cty=JWT` / `cty=jwt` / その他の `cty`、`alg=none`、`kid` 欠落、改行入り JWT | 未知 Header は無視し、`cty` を含む未対応の形式・拡張を拒否 |
| JWKS | `kid` 不在、除外した JWK だけが一致、複数一致、2048 bit 未満の法、`use`・`key_ops`・`alg` の不整合、公開指数の境界、`x5c`・Key Usage の不整合、2 個以上の証明書を含む `x5c` の署名の関係、鍵ゼロ、秘密情報混入、取得障害 | GOJ-JWKS-FORMAT、GOJ-JWKS-SECRET、GOJ-JWKS-USABLE、GOJ-JWKS-KID、GOJ-SIGNATURE の分類に一致し、すべて Fail Closed |

## References

- [RFC 7515 JSON Web Signature](../external/standards/rfc7515.md) — `RFC7515-` の条項
- [RFC 7517 JSON Web Key](../external/standards/rfc7517.md) — `RFC7517-` の条項
- [RFC 7518 JSON Web Algorithms](../external/standards/rfc7518.md) — `RFC7518-` の条項
- [RFC 7519 JSON Web Token](../external/standards/rfc7519.md) — `RFC7519-` の条項
- [RFC 7797 JWS Unencoded Payload Option](../external/standards/rfc7797.md) — `RFC7797-` の条項
- [RFC 8725 JWT Best Current Practices](../external/standards/rfc8725.md) — `RFC8725-` の条項
- [RFC 9964 ML-DSA for JOSE and COSE](../external/standards/rfc9964.md) — `RFC9964-` の条項
- [OpenID Connect Core 1.0](../external/standards/oidc-core.md) — `OIDCC-` の条項
- [OpenID Connect Discovery 1.0](../external/standards/oidc-discovery.md) — `OIDCD-` の条項
- [GitHub Actions OIDC Constraints](../external/github/oidc.md) — `GH-OIDC-` の条項
- [GitHub OIDC Discovery](../research/github/oidc-discovery.md) — Discovery の公開値
- [GitHub OIDC JWKS](../research/github/oidc-jwks.md) — 鍵情報
- [RFC 7517 — JSON Web Key](https://www.rfc-editor.org/rfc/rfc7517.html)（`x5c` の証明書の規定）
