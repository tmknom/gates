# JOSE Standards

この文書は、JWS / JWK / JWA の標準仕様と JWS の非エンコード Payload オプションから、署名と鍵の検証に関係する規定を整理したものです。標準仕様の代替ではなく、規定の適用対象と条件はリンク先の原文を優先します。

- **対象**：JWS Compact Serialization、RSA 署名、JOSE Header、JWK と JWK Set に関係する規定
- **対象外**：JWE による暗号化、各署名方式の網羅的な解説、OpenID Connect が追加する規定

RFC の **MUST / SHOULD / MAY** は原文の規範的強度を表します。

## 1. JWS and JOSE Header

### 1.1 構造、署名、アルゴリズム

**標準仕様**：JWT は JWS Compact Serialization か JWE Compact Serialization で表します（[RFC 7519 §1](https://www.rfc-editor.org/rfc/rfc7519.html#section-1)）。
JWS Compact Serialization は、Base64URL エンコードされた Protected Header、Payload、Signature の3部分を `.` で連結します。
JOSE Header の `alg` は、存在することと、理解・処理されることが **MUST** です。
`alg` が対応しないアルゴリズムを表す場合や、署名者に結び付くそのアルゴリズム用の鍵がない場合、署名は無効です。
署名は、`alg` が正確に表す方式で検証します（[RFC 7515 §3.1、§4.1.1、§5.2](https://www.rfc-editor.org/rfc/rfc7515.html#section-5.2)）。

**標準仕様**：`RS256` は RSASSA-PKCS1-v1_5 と SHA-256 の組み合わせであり、この方式で使う RSA 鍵は2048 bit 以上を **MUST** とします。`PS256` は別の署名方式です（[RFC 7518 §3.3、§3.5](https://www.rfc-editor.org/rfc/rfc7518.html#section-3.3)）。

**標準仕様**：署名を検証できても、アルゴリズムをアプリケーションが受け入れないなら、JWS を無効とみなすことが **SHOULD** です（[RFC 7515 §5.2](https://www.rfc-editor.org/rfc/rfc7515.html#section-5.2)）。

### 1.2 鍵の識別と Header の拡張

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

**標準仕様**：発行側は、`crit` に空配列、重複、JOSE Header に存在しない名前、JWS / JWA が定義済みの名前を含めてはいけません（**MUST NOT**）。`crit` は JWS Protected Header に置き、完全性保護を受けることが **MUST** です。受信者は、これらの条件への違反を理由に JWS を無効とみなすことができます（**MAY**）。未対応の拡張を含む場合の無効化とは規範の強さが異なります（[RFC 7515 §4.1.11](https://www.rfc-editor.org/rfc/rfc7515.html#section-4.1.11)）。

**標準仕様**：JWT に JWS の非エンコード Payload オプション（`b64:false`）を使用することは **MUST NOT** です。
`b64` を使う場合は、`crit` に `b64` を列挙することが **MUST** です（[RFC 7797 §6–§7](https://www.rfc-editor.org/rfc/rfc7797.html#section-7)）。

## 2. JWK and JWK Set

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

## 3. References

- [RFC 7515 — JSON Web Signature (JWS)](https://www.rfc-editor.org/rfc/rfc7515.html)（2015、§3–§6）
- [RFC 7517 — JSON Web Key (JWK)](https://www.rfc-editor.org/rfc/rfc7517.html)（2015、§4–§5）
- [RFC 7518 — JSON Web Algorithms (JWA)](https://www.rfc-editor.org/rfc/rfc7518.html)（2015、§3.3、§3.5、§6.3）
- [RFC 7797 — JSON Web Signature (JWS) Unencoded Payload Option](https://www.rfc-editor.org/rfc/rfc7797.html)（2016、§6–§7。RFC 7519 の更新）
