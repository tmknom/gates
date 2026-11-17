# JWT Standards

この文書は、JWT、JSON、JWT の Best Current Practices の標準仕様から、Claims の検証と JWT の発行・検証の考慮事項に関係する規定を整理したものです。標準仕様の代替ではなく、規定の適用対象と条件はリンク先の原文を優先します。

- **対象**：JWT Claims、JSON の文字列比較、JWT の発行と検証のセキュリティ上の考慮事項に関係する規定
- **対象外**：OAuth 2.0 の認可フロー全体、JWE による暗号化、OpenID Connect が ID Token に追加する規定

RFC の **MUST / SHOULD / MAY** は原文の規範的強度を表します。

## 1. JWT Claims

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

## 2. JWT Security Considerations

**標準仕様**：RFC 8725 は、JWT の発行と検証に関して次の点を定めています。

- **UTF-8**：Header と Claims Set の JSON には UTF-8 を使い、ほかの Unicode 符号化を認めないことが **MUST**（[§3.7](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.7)）
- **Issuer と Subject**：`iss` があれば、使う鍵がその Issuer に属することを確かめ、属さなければ拒否することが **MUST**。`sub` があれば、アプリケーションにとって有効な値か確かめ、無効なら拒否することが **MUST**（[§3.8](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.8)）
- **Audience**：同じ Issuer が複数の受信者向けに JWT を発行し得る場合、JWT は `aud` を持つことが **MUST**。受信者は `aud` を検証し、欠落や不一致なら拒否することが **MUST**（[§3.9](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.9)）
- **受信した値**：`kid` はインジェクションを招かないよう検証または無害化する。`jku` と `x5u` の URL をそのままたどらず、SSRF を防ぐことが **SHOULD**（[§3.10](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.10)）
- **JWT の種類の区別**：同じ Issuer が複数の種類の JWT を発行し得る場合、検証規則は互いに排他的であることが **MUST**。手段には `typ`、必須 Claim、Header、鍵、`aud`、Issuer の違いがある。新しい用途では `typ` による明示が **RECOMMENDED**（[§3.11–§3.12](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.11)）

**標準仕様**：JWT ライブラリは、対応するアルゴリズムの集合を呼び出し元が指定できるようにし、それ以外を暗号操作に使ってはいけません（**MUST NOT**）。
ライブラリは `alg` が実際の暗号操作と同じ方式を示すことを確かめます（**MUST**）。
各鍵は一つのアルゴリズムだけで使い、そのことを暗号操作時に確かめます（**MUST**）。
アプリケーションは、セキュリティ要件を満たす現行のアルゴリズムだけを許可します（**MUST**）（[RFC 8725 §3.1–§3.2](https://www.rfc-editor.org/rfc/rfc8725.html#section-3.1)）。

## 3. References

- [RFC 7159 — The JavaScript Object Notation (JSON) Data Interchange Format](https://www.rfc-editor.org/rfc/rfc7159.html)（2014、§8.3）
- [RFC 7519 — JSON Web Token (JWT)](https://www.rfc-editor.org/rfc/rfc7519.html)（2015、§2、§4、§7.3）
- [RFC 8725 — JSON Web Token Best Current Practices](https://www.rfc-editor.org/rfc/rfc8725.html)（2020、§3。RFC 7519 の更新）
