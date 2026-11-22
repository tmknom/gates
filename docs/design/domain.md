# Domain Model

GATES が扱う問題領域のドメインモデルです。
GATES は、許可された単一のオーナー配下の GitHub Actions ワークフローに、明示されたリポジトリと明示された Repository permissions の権限に限定した短命な GitHub App Installation Access Token を発行します。

## Purpose and Scope

本ドキュメントの目的は、Lambda 上の Go アプリケーション、Docker Action、Terraform の設計・実装・レビューが、概念の意味と判定のルールを一つのモデルから参照できるようにすることです。
中心に置くのは、Lambda が担う発行の判定です。
Docker Action、AWS IAM、KMS の鍵管理は、発行の判定と語の意味が異なる範囲として境界を示すにとどめます。

モデルが答える問いは次のとおりです。

- 発行要求を誰が出し、何を要求し、何が返るか
- オーナー、リポジトリ、Installation を何で同定するか
- 発行を許可する条件は何か。拒否はどの段階で、どの種別として起きるか
- どの単位の中で、何が常に成り立たなければならないか
- 境界をまたぐと、同じ語の意味がどう変わるか

概念には英語名を併記します。
英語名は概念の名前であり、複数の単語は空白で区切ります（Token Request など）。
実装では空白を除いた形（TokenRequest など）を型名・関数名にし、実装と本ドキュメントの整合はこの対応で確かめます。
package 名が概念名の一部を表すときは、型名からその部分を除きます（Revocation Outcome を revocation package の Outcome に、Signature-Verified Claims Set を claimsset package の SignatureVerified に、Success Response を response package の Success にするなど）。
Lambda のコードに現れる OIDC JWT は GATES 用 OIDC JWT だけのため、型名から GATES を除きます（Unverified GATES OIDC JWT を UnverifiedOIDCJWT にするなど）。

本ドキュメントは、GATES に必要な値を概念として定義し、GitHub や AWS がその値をどのフィールドや Claim で提供するかは書きません。
提供のされ方は手段であり、概念の意味を変えないためです。
GitHub が定めた値そのものを書くのは、ルールがその値に依存する付随権限に限ります。
GitHub が定める仕様の内容、出典、未確認事項は [GitHub Actions OIDC Constraints](../external/github/oidc.md)、[GitHub Owner and Repository Constraints](../external/github/account.md)、[GitHub App Constraints](../external/github/app.md) を正本とし、本ドキュメントはそれが GATES のルールに及ぼす影響だけを書きます。
JWT / JWS / JWK / OpenID Connect の標準仕様の規定の整理は、[External](../external/README.md) に挙げた標準仕様の文書に置きます。

本ドキュメントの記述は、根拠の種類を次の表記で区別します。

- **事実**：情報源に根拠がある。出典を添える
- **推論**：根拠から導いた判断
- **候補**：本ドキュメントが補った語や区分で、情報源にはない
- **未確認**：情報源で決まっていない、または情報源どうしで食い違う

## Problem Overview

GitHub Actions の `GITHUB_TOKEN` は、ワークフローが動くリポジトリの外を読めません。
別のリポジトリを読むには、GitHub App の Installation Access Token が要ります。
しかし、GitHub App の Private Key をワークフローに渡すと、鍵を持つ者が App の権限全体を使えてしまいます。

GATES は、この二つの要求を同時に満たす仲介者です。
鍵は AWS KMS に置いて署名だけを行わせ、ワークフローには、本人確認を経た上で、明示された対象と権限だけに絞ったトークンを渡します。
問題領域の関係者は 3 者です（事実：Requirements の Stakeholders and Responsibilities）。

- **ワークフロー利用者**：対象リポジトリと権限を指定してトークンを受け取る
- **GitHub App / オーナーの管理者**：Installation の対象リポジトリを管理し、GATES で読める範囲の上限を決める
- **GATES 運用者**：許可オーナー、GitHub App、署名鍵、発行上限、保護対象の一覧を固定し、GATES を運用する

GATES が所有する概念は、固定設定、発行要求、Caller、発行許可、発行結果、監査記録です。
オーナー、リポジトリ、GitHub App、Installation は GitHub が所有する概念で、GATES はそれらを参照し、照合します。

GATES は、発行の判定を Invoke ごとの **発行処理**（Issuance Process）で行います。
発行処理は、Lambda Handler が一回の Invoke の相関 ID を確かめてから、成功応答か失敗応答を書くまでの処理です（事実：Lambda Design §3.2）。
監査記録、失敗の種別、[INV-014 発行処理](#inv-014-発行処理)は、どれも発行処理 1 回を単位に定めます。

## Concepts

### GitHub Entities Referenced by GATES

| 概念 | 英語名 | 同一性 | 主な性質 | 根拠 |
|---|---|---|---|---|
| **オーナー** | Owner | 不変 Owner ID | 名前（変更され得る）、種別（Organization またはユーザーアカウント）。リポジトリを所有する | 事実：GitHub Owner and Repository Constraints |
| **リポジトリ** | Repository | 不変 Repository ID | 完全名 `owner/repo`（名前変更・移管で変わり得る）、所有オーナー | 事実：GitHub Owner and Repository Constraints、GitHub Actions OIDC Constraints |
| **GitHub App** | GitHub App | Client ID | 権限の上限（GitHub 側で設定）、Private Key（複数持てる。上限は GitHub App Constraints） | 事実：GitHub App Constraints |
| **Installation** | Installation | Installation ID | インストール先 Owner ID、対象の選び方、付与された権限、停止状態 | 事実：GitHub App Constraints |
| **ワークフロー実行** | Workflow Run | ワークフロー実行 ID と試行番号 | 実行元のリポジトリとオーナー、起動のイベントと ref など。GATES 用 OIDC JWT の主体は実行元とワークフロー起動属性の一部を表し、実行そのものは識別しない | 事実：GitHub Actions OIDC Constraints |
| **Issuer の公開鍵集合** | JWK Set（JWKS） | 固定 Issuer に対応する公開鍵集合（取得先は Discovery の `jwks_uri`） | 署名検証用の個々の JWK の集合。ローテーションによって内容が変わり得る。GATES は Discovery で取得先を確認し、使用可能な鍵だけを実行環境内で保持する。個々の鍵の選択には JWK の `kid` を使う。秘密鍵か対称鍵の値を含む集合は、全体を受け入れない | 事実：Lambda Design §6.1、RFC 7517 §4.5・§5。日本語の概念名は本ドキュメントの表現 |
| **Discovery** | Discovery | Issuer | Issuer が公開する設定。GATES はそこから Issuer の値と公開鍵の取得先だけを読む。成功した内容は実行環境の中で再利用し、失敗した内容は保持しない | 事実：Lambda Design §6.1、OpenID Connect Discovery |

オーナーとリポジトリは、どちらも不変 ID と名前の二つの表現を持ちます。
名前は変わり得るため、認可の同一性は不変 ID で判断し、名前は、要求の照合、発行前の保護対象の判別、発行結果の照合、許可 Caller の照合（ID の一致に重ねる条件）、GitHub API の呼び出しに使う表現と位置づけます（事実：Requirements FR-AUTHZ-001、FR-AUTHN-001、FR-AUTHZ-009、FR-AUTHZ-010、FR-ISSUANCE-004）。
名前が変わると、ID で同じものを指していても名前の照合は失敗します。
この失敗は、GATES では Fail Closed として受け入れます（事実：AWS Infrastructure Design §3.1）。

名前の表現は、次の値で扱います。

| 概念 | 英語名 | 種別 | 内容 | 根拠 |
|---|---|---|---|---|
| **完全名** | Full Name | 値 | `owner/repo` 形式のリポジトリ名。オーナー名とリポジトリ名に分割でき、比較は ASCII の範囲で大文字小文字を区別しない | 事実：Lambda Design §2.2 |
| **オーナー名** | Owner Name | 値 | 完全名のオーナー部分と許可オーナー名の二か所に現れるオーナーの名前。どちらも同じ文字種の規則に従う。比較は ASCII の範囲で大文字小文字を区別しない | 事実：Lambda Design §2.2 |
| **リポジトリ名** | Repository Name | 値 | オーナー名を含まないリポジトリの名前。完全名のリポジトリ部分、保護対象と許可 Caller の名前、Caller のリポジトリ名に現れる。完全名のリポジトリ部分、保護対象のリポジトリ名、許可 Caller のリポジトリ名は、同じ文字種の規則に従う。Caller のリポジトリ名には文字種を求めない。比較は ASCII の範囲で大文字小文字を区別しない | 事実：Lambda Design §2.2、Requirements FR-AUTHZ-009。保護対象のリポジトリ名に同じ文字種を課すのは推論：要求の名前と照合する値で、規則の外の名前は要求の側に現れないため。許可 Caller のリポジトリ名に課すのは推論：規則の外の名前の Caller を、どの許可 Caller にも当てずに拒否に倒すため。Caller のリポジトリ名に求めない理由は [INV-009 Caller](#inv-009-caller) の根拠 |

Installation には、GATES が参照する次の属性があります。

- **インストール先 Owner ID**（Installation Owner ID）：Installation がインストールされているオーナーの不変 Owner ID
- **停止状態**（Suspended）：オーナーの管理者が Installation を停止しているかどうか

Installation の対象の選び方と付与された権限は、GATES の判定に使いません。
対象の選び方は、発行結果の属性としてだけ照合します（[Issuance Result](#issuance-result)）。

Installation は、GitHub App とオーナーの組に対して高々一つ存在します（推論：GitHub App Constraints の、オーナーを指定して単一の Installation を返すエンドポイントが Organization とユーザーアカウントのそれぞれにあることから。GitHub が一意性を明示的に保証したものとは扱わない）。
この性質があるため、GATES は Installation ID を設定に持たず、固定した GitHub App と許可オーナーから実行時に導出します（事実：Requirements FR-OPS-001）。

Issuer の公開鍵集合の JWK は、GATES の受理条件で次の二つに分かれます。

- **使用可能な鍵**（Usable Key）：JWK の受理条件を満たし、RSA 公開鍵として署名の検証に使える JWK。署名の一致は意味しない（事実：GATES OIDC JWT Signature の GOJ-JWKS-USABLE）
- **除外した JWK**（Excluded JWK）：受理条件を満たさず保持しない JWK。`kid` も保持しないため、JWT の判定では存在しない鍵と同じに扱う。運用者が Issuer の鍵情報の不整合に気付けるよう記録するが、監査記録の項目ではない（事実：GATES OIDC JWT Signature の GOJ-JWKS-USABLE。英語名は候補。監査記録は発行処理 1 回の単位で、除外は取得 1 回の単位のため）

ワークフロー実行は、**ワークフロー実行 ID**（Run ID）と **試行番号**（Run Attempt）の組で特定します。
再実行は同じワークフロー実行 ID のもとで試行番号を増やすため、両方を持って初めて一回の実行を指せます（事実：GitHub Actions OIDC Constraints）。

### Fixed Configuration

**固定設定**（Config）は、実行時の入力からは変更できない値の集まりです（事実：Requirements FR-OPS-001、FR-AUTHZ-002）。
Issuer と API Host は GATES 自身が固定し、残りの値は GATES 運用者がデプロイ時に決めます（事実：AWS Infrastructure Design §5）。
そのうち許可オーナーに関する 3 つの値を **許可オーナー**（Allowed Owner）と呼びます。

| 値 | 英語名 | 使われ方 | 根拠 |
|---|---|---|---|
| 許可オーナーの不変 Owner ID | Allowed Owner ID | Caller の認可、インストール先 Owner ID の照合、実効対象の所有オーナーの Owner ID の照合 | 事実：Requirements FR-AUTHZ-001、Security Design §3.1 |
| 許可オーナー名 | Allowed Owner Name | 要求された完全名のオーナー部分の照合、Installation の取得先の指定 | 事実：Lambda Design §5 手順 3、§6.3 |
| 許可オーナーの種別 | Allowed Owner Type | Installation の取得方法の選択（Organization かユーザーアカウントか） | 事実：Lambda Design §6.3 |
| GitHub App の Client ID | Client ID | GitHub App JWT の発行者 | 事実：Lambda Design §4 |
| 署名鍵の KMS Key ARN | KMS Key ARN | GitHub App JWT の署名 | 事実：Lambda Design §4 |
| 発行上限 | Issuable Permissions | 事前認可と発行許可の確定で、要求の Permissions を照合する（[Request and Decision](#request-and-decision)） | 事実：Requirements FR-AUTHZ-008 |
| 保護対象の一覧 | Protected Repositories | 事前認可と発行許可の確定で、保護対象に当たる対象への要求の Caller と Permissions を照合し、発行結果の照合で実効対象のリポジトリ名と Repository ID の対応を照合する（[Protected Repositories](#protected-repositories)） | 事実：Requirements FR-AUTHZ-009、FR-AUTHZ-010 |
| GATES 用 Audience | Audience | GATES 用 OIDC JWT の Audience の照合 | 事実：GATES OIDC JWT Claims の GOJ-AUD |
| Issuer | Issuer | GATES 用 OIDC JWT の Issuer の照合と Discovery / JWKS の取得先。GitHub Actions の OIDC Provider に固定。JWKS は Issuer の Discovery から得るため、独立した値を持たない | 事実：GATES OIDC JWT Claims の GOJ-ISS、GATES OIDC JWT Signature の GOJ-DISCOVERY-ISSUER、GOJ-DISCOVERY-JWKS-URI、Lambda Design §6.1 |
| GitHub API Host | API Host | Installation の取得、トークンの発行と失効の呼び出し先。GitHub.com の API に固定 | 事実：Requirements FR-AUTHZ-002、Scope §4、Lambda Design §6.3 |

Authorization が読むのは、固定設定のうち許可オーナー（ID と名前）、発行上限、保護対象の一覧です。
発行結果の照合も、許可オーナーと保護対象の一覧を読みます。
Client ID、KMS Key ARN、Audience、Issuer、API Host は、認証、署名、Installation の解決、発行を担うコンポーネントが読みます。

### Request and Decision

| 概念 | 英語名 | 種別 | 内容 | 根拠 |
|---|---|---|---|---|
| **発行要求** | Token Request | 値 | 対象（重複のない完全名の列）と Permissions。利用者が明示する。対象は明示した完全名だけで、全リポジトリを対象にする手段を持たない | 事実：Requirements FR-AUTHZ-003、Lambda Design §2.1 |
| **Permission Entry** | Permission Entry | 値 | 権限名と水準の組（`contents:read` など） | 事実：Requirements FR-AUTHZ-006 |
| **Permissions** | Permissions | 値 | Permission Entry の集合。権限名ごとに水準を一つ持つ（GitHub の権限の表現が権限名をキーに持つため）。要求では、どの Permission Entry も許可権限に含まれ、発行上限に[水準の包含](#permission-levels)で含まれるものだけが許可される。保護対象に当たる対象を含む要求では、許可 Caller の権限の上限にも水準の包含で含まれる必要がある。GitHub の Installation Access Token の権限の表現に対応する。権限名は、発行 API のスキーマ「App Permissions」が定めるものに限らない | 事実：Requirements FR-AUTHZ-006、Security Design §3.1、GitHub App Constraints |
| **許可権限** | Allowed Permissions | 値 | GATES が発行を許可する Permission Entry の集合。GitHub が Repository permissions に分類する権限の権限名と、GitHub が要求どおりに発行する水準の組を、GATES が列挙して固定したもの。発行 API のスキーマに権限名があることを条件にしない。一つの権限名に複数の水準を含み得るため、Permissions ではない。Caller と対象によって変わらない | 事実：Requirements FR-AUTHZ-006。列挙する権限名と水準：GitHub App Constraints の「権限名と分類」 |
| **発行上限** | Issuable Permissions | 値 | 許可権限のうち、そのデプロイで発行してよい Permission Entry の集合。GATES 運用者が固定設定として決める。許可権限と同じく一つの権限名に複数の水準を含み得るため、Permissions ではない。要求の Permission Entry は、発行上限に水準の包含で含まれるかを照合する。Caller と対象によって変わらない | 事実：Requirements FR-AUTHZ-008 |
| **付随権限** | Implied Permissions | 値 | 要求にかかわらず GitHub が付与する Permission Entry。`metadata:read` だけを許容する。発行上限にも、保護対象を含む要求で Caller が当たる許可 Caller の権限の上限にも含まれなくても許容する | 事実：Requirements FR-ISSUANCE-004 が除外の枠を定める。`metadata:read` だけを許容するのは GATES の判断。許可権限の各権限名を単独で要求した発行と、複数の権限を要求した発行では、`metadata:read` のほかに加わった権限はなかった（GitHub App Constraints）。未確認：`metadata:read` が付与される条件と、ほかの権限が加わらないことの保証。語は候補 |
| **Caller** | Caller | 値 | 検証済み GATES 用 OIDC JWT から確定した呼び出し元。不変 Owner ID と Repository ID で識別し、リポジトリ名を持つ。ワークフロー実行 ID と試行番号、およびワークフロー起動属性を監査のためだけに添える | 事実：Requirements FR-AUTHN-001 |
| **ワークフロー起動属性** | Workflow Trigger Attributes | 値 | ワークフロー実行の起動と、GATES 用 OIDC JWT を取得したジョブを示す属性の組。誰が起動したか（行為者の名前と ID）、どのイベントか、どの ref とコミットか、どのワークフロー定義か、どの environment か。どれも欠落し得る | 事実：GitHub Actions OIDC Constraints、GATES OIDC JWT Claims の Claim Summary |
| **署名検証済みの Claims Set** | Signature-Verified Claims Set | 値 | 署名検証を通った GATES 用 OIDC JWT の Claims Set の JSON オブジェクト全体。Claim の分類によらずすべての Claim を含み、署名を含まない。監査記録で調査に使うだけで、Caller にもワークフロー起動属性にも含めない | 事実：Requirements FR-OPS-003 |
| **発行許可** | Token Grant | 値 | Installation ID、対象の完全名の列、発行要求の Permissions。認可が成立した結果であり、発行 API の呼び出しの内容を定める | 事実：Security Design §3.1 |

許可権限と発行上限は、どちらも要求の Permissions を照合する Permission Entry の集合ですが、決める主体と変わる契機が異なるため、別の概念として扱います。
許可権限は GATES のコードが列挙して固定し、GitHub の分類と発行の振る舞いを確かめた結果でだけ変わります（事実：Security Design §3.3）。
発行上限は GATES 運用者がデプロイごとに決め、許可権限の一部に限られます（事実：Requirements FR-AUTHZ-008）。
一つの概念にまとめると、デプロイ時の設定で Organization permissions のような許可権限の外の Permission Entry を発行できる形になり、許可権限が最後の上限として働かなくなります（推論）。
付随権限は要求の照合ではなく発行結果の照合で許容するものであり、発行上限の影響を受けません（事実：Security Design §3.3）。

発行要求と Caller は、同じ Invoke Payload から取り出す別の概念です。
Payload に含まれる GATES 用 OIDC JWT は Caller を確定するための資格情報であり、発行要求の一部ではありません（事実：Lambda Design §2.1）。

Caller は、利用者の自己申告からは作れません（事実：Requirements FR-AUTHN-001、FR-AUTHZ-002）。
Caller の Owner ID は認可に使い、Repository ID は監査の識別と保護対象の認可に使います（事実：Requirements FR-AUTHN-001、FR-AUTHZ-009）。
Caller のリポジトリ名は保護対象の認可だけに使い、Caller の同一性には含めません（事実：Requirements FR-AUTHN-001。同一性に含めないのは推論：名前は改名で変わり、作り直したリポジトリが同じ名前を持ち得るため）。
ワークフロー実行 ID、試行番号、ワークフロー起動属性は監査の識別だけに使います（事実：Requirements FR-AUTHN-001）。
ワークフロー実行 ID と試行番号を添えるのは、監査記録から GitHub 上のワークフロー実行にたどり着けるようにするためで、Caller の同一性には含めません（事実：Security Design §5）。
ワークフロー起動属性を添えるのは、`pull_request_target` の悪用のように、どのイベントと ref でトークンを得たかが調査に要る場面で、GitHub 側の記録を参照せずに済ませるためです（事実：Security Design §5）。
ワークフロー起動属性はベストエフォートの属性であり、欠落していても Caller は確定します。
ワークフロー起動属性の値は監査記録に書き写すだけで、GATES は解釈も検証もせず、欠落した属性は欠落のまま持ちます（推論：認可に使わないため）。
値が空文字列の属性は、欠落と同じに扱います（推論：空の値は調査の手がかりにならず、区別する要求がないため）。
値が文字列でない属性も、欠落と同じに扱います（推論：拒否すると、認可に使わない属性の形が変わっただけで発行が止まるため。代わりに、GitHub が属性の型を変えると監査記録から値が消え、変更に気付きにくくなることを受け入れる）。

ワークフロー起動属性は意味を確かめて選んだ Claim の組で、後から調査に要る Claim をすべては予測できません。
そのため、署名検証済みの Claims Set を、ワークフロー起動属性とは別に監査記録へ残します（事実：Requirements FR-OPS-003）。
署名検証済みの Claims Set は資格情報ではありません（推論：署名を含まず、GATES は署名のない Claims Set を受理しないため、Bearer として提示できない）。
GATES は署名検証済みの Claims Set を監査記録に書き写すだけで、認証、認可、ワークフロー起動属性のどの判断にも使いません（推論：未知の Claim や `repo_property_*` の意味、型、変更権限を評価していないため）。

発行許可は、Caller、発行要求、固定設定（許可オーナー、発行上限、保護対象の一覧）、取得した Installation がそろい、[RULE-010 認可条件](#rule-010-認可条件)、[RULE-015 保護対象の認可条件](#rule-015-保護対象の認可条件)、[RULE-011 Installation の有効性](#rule-011-installation-の有効性)をすべて満たしたときにだけ作られます。
発行許可があって初めて GitHub API へ発行を要求できる、という順序がこの概念の役割です。

### Permission Levels

発行上限と許可 Caller の権限の上限（[Protected Repositories](#protected-repositories)）は、どちらも要求の Permission Entry を、同じ権限名の水準を比べて照合します。
GitHub App に付与した水準より低い水準の要求は要求どおりに発行されたため、照合も高い水準が低い水準を含むものとします（事実：Requirements FR-AUTHZ-008、GitHub App Constraints §4.4 の観測。GitHub はこの振る舞いを保証していない）。

- **水準の順序**（Level Order）：同じ権限名の中で `read`、`write`、`admin` の順に高い。異なる権限名の水準は比べない
- **水準の包含**（Level Inclusion）：Permission Entry の集合を上限として、上限が Permission Entry `p:l` を含むのは、`p:l` が許可権限に含まれ、かつ上限が同じ権限名 `p` の、`l` と同じかより高い水準の Permission Entry を含むときである

`write` だけを許可権限に含む権限名があれば、上限にその `write` を書いても、許可権限にない `read` の要求は許しません（推論：許可権限を最後の上限として保つため。水準を比べるだけでは、GATES が要求どおりに発行されると確かめていない水準を許し得る）。
水準の包含は要求を許すかだけを決め、発行する水準を変えません。
発行許可の Permissions は要求の Permissions のままで、上限が `contents:write` でも `contents:read` の要求には `contents:read` を発行します（事実：Requirements FR-AUTHZ-008）。
発行結果の照合は、要求した水準との一致を求めたままです（[INV-012 返却可能な発行結果](#inv-012-返却可能な発行結果)）。

### Protected Repositories

GATES 運用者は、操作できるワークフローを一部のリポジトリのものに限りたいリポジトリを、固定設定で指定できます（事実：Requirements FR-AUTHZ-009）。
指定したリポジトリへの要求には、オーナー単位の認可条件に加えて、そのリポジトリが許す Caller と権限の条件が課されます（事実：Security Design §3.6）。

| 概念 | 英語名 | 種別 | 内容 | 根拠 |
|---|---|---|---|---|
| **保護対象** | Protected Repository | 値 | 要求できる Caller と権限を限るリポジトリ。Repository ID、リポジトリ名、許可 Caller の集合を持つ。許可 Caller の集合は空でもよい | 事実：Requirements FR-AUTHZ-009 |
| **許可 Caller** | Allowed Caller | 値 | 保護対象への要求を許す Caller のリポジトリと、その Caller に許す権限の組。Repository ID、リポジトリ名、権限の上限を持つ | 事実：Requirements FR-AUTHZ-009 |
| **権限の上限** | Permission Limit | 値 | 許可 Caller に許す Permission Entry の上限。Permissions と同じく権限名ごとに水準を一つ持ち、要求の Permission Entry を[水準の包含](#permission-levels)で照合する | 事実：Requirements FR-AUTHZ-009 |
| **保護対象の一覧** | Protected Repositories | 値 | 保護対象の集合。固定設定の値で、空でもよい | 事実：Requirements FR-AUTHZ-009、FR-OPS-001 |

保護対象の同一性は Repository ID です（事実：Requirements FR-AUTHZ-010）。
リポジトリ名を併記するのは、発行要求が対象を完全名だけで示すため、発行前には名前でしか保護対象を見分けられないからです（推論：Lambda Design §2.1 の発行要求の形から）。
名前で見分けた結果は、発行結果の照合で、実効対象の Repository ID と突き合わせます。
名前と ID の対応が一覧と GitHub で食い違えば、その発行結果は返しません（[INV-012 返却可能な発行結果](#inv-012-返却可能な発行結果)）。
こうして、保護対象を改名して新しい名前で要求しても、保護を外せません（推論：新しい名前は一覧の名前と一致せず、実効対象の ID は保護対象の ID と一致するため）。

許可 Caller の同一性も Repository ID です。
許可 Caller には、Repository ID の一致に重ねて、リポジトリ名が Caller のリポジトリ名と一致することを求めます（事実：Requirements FR-AUTHZ-009）。
名前を照合するのは、一覧を読む人が名前で許可を判断するためです。
照合しなければ、ID を書き誤った一覧は、名前の示すリポジトリとは別のリポジトリを許可したまま誰にも気付かれません（推論）。
照合すれば、ID と名前のどちらを書き誤っても、その許可 Caller はどの Caller にも当たらなくなり、拒否に倒れます。

保護対象の一覧は、許可権限と発行上限のどちらとも別の概念です。
許可権限と発行上限は Caller と対象によって変わらず、固定設定のうち Caller と対象の組で条件を変えるのは保護対象の一覧だけであるためです（事実：Requirements FR-AUTHZ-006、FR-AUTHZ-008、Security Design §3.1）。
保護対象の一覧は、ほかの認可条件、Installation の対象範囲と有効性、GitHub App に付与された権限の条件で拒否される要求を許可に変えません（事実：Requirements FR-AUTHZ-009）。
許可 Caller の権限の上限は、許可権限の一部に限りますが、発行上限の一部であることは求めません（推論：求めると、発行上限を狭めただけで固定設定が不変条件を満たさなくなって初期化に失敗し、どの要求も発行できなくなる。発行上限の外の Permission Entry は [RULE-010 認可条件](#rule-010-認可条件)が拒否するため、許す範囲は広がらない）。

### Issuance Result

| 概念 | 英語名 | 種別 | 内容 | 根拠 |
|---|---|---|---|---|
| **発行結果** | Issuance Result | 値 | GitHub API が返した Installation Access Token、有効期限、実効対象の集合、実効権限、対象の選び方。GitHub REST API のスキーマ「Installation Token」に対応する | 事実：GitHub App Constraints、REST API endpoints for GitHub Apps のスキーマ「Installation Token」 |
| **実効権限** | Effective Permissions | 値 | 発行結果に含まれる Permissions。発行許可の Permissions に付随権限を加えた範囲内で、発行許可の Permissions をすべて含むことを照合する | 事実：Requirements FR-ISSUANCE-005、FR-ISSUANCE-004 |
| **実効対象** | Effective Repository | 値 | 発行結果に含まれる各リポジトリ。Repository ID、所有オーナーの Owner ID、完全名を持つ | 事実：Lambda Design §6.3 |
| **返却可能な発行結果** | Returnable Issuance Result | 値 | 発行許可との照合を通った発行結果。利用者に返せるのはこれだけ。トークン、有効期限、実効権限、実効対象の集合を持つ | 事実：Security Design §3.4 |

**対象の選び方**（Repository Selection）は、発行結果の対象が選択したリポジトリに限られるか、全リポジトリかの二値です。

Installation Access Token はトークン文字列そのもので、GitHub の名前です。
照合の条件は [INV-012 返却可能な発行結果](#inv-012-返却可能な発行結果)に置きます。

発行結果は照合の前の値であり、不変条件を持ちません。
各項目は発行応答から独立に読み取り、読み取れなかった項目は、読み取れなかったことが分かる形で発行結果に残します（事実：Lambda Design §6.3）。
照合は、読み取れなかった項目に関わる条件を満たさないものとして扱います。
こうすると、トークン文字列以外の項目を読み取れたかどうかは照合の結果だけを変え、トークン文字列の取り出しと、失効を試みられるかどうかを変えません。
実効対象の集合は、すべての要素について Repository ID、所有オーナーの Owner ID、完全名を読み取れた場合にだけ、読み取れたものとします（推論：一部の要素だけを持っても、全件の一致を求める照合の結果は変わらないため）。

### Credentials

GATES の処理には 5 種類の **資格情報**（Credential）が現れ、どれも「トークン」と呼ばれ得ます（事実：System Architecture §4）。
語の衝突を避けるため、本ドキュメントでは次の名前で区別します。

| 資格情報 | 英語名 | 発行元 | 用途 | 発行の判定に現れるか |
|---|---|---|---|---|
| **AWS 用 OIDC JWT** | AWS OIDC JWT | GitHub OIDC Provider（Audience は AWS STS） | IAM Role の引き受け | 現れない。利用者接点が取得し、入口制御が検証する |
| **AWS 一時認証情報** | AWS Credentials | AWS STS | Lambda Invoke の署名 | 現れない |
| **GATES 用 OIDC JWT** | GATES OIDC JWT | GitHub OIDC Provider（Audience は GATES 用 Audience） | Caller の確定 | 現れる。認証の入力 |
| **GitHub App JWT** | App JWT | Lambda が組み立て、KMS が署名 | GitHub API の呼び出し | 現れる。内部でだけ使い、外に出さない |
| **Installation Access Token** | Installation Access Token | GitHub API | 対象リポジトリの、実効権限が許す操作 | 現れる。発行結果の中心。返却可能な発行結果はこれに有効期限・対象・実効権限を添えたもの |

GATES 用 OIDC JWT は、Payload から取り出した時点では未検証であり、認証を通って初めて Caller の根拠になります。
認証を通る前の値を **未検証の GATES 用 OIDC JWT**（Unverified GATES OIDC JWT）と呼びます。
GATES 用 OIDC JWT は OpenID Connect Core の ID Token ではありません（事実：GATES OIDC JWT Claims の「ID Token として扱わないこと」）。
Invoke Payload の `id_token` やエラーコード `invalid_id_token` は Docker Action との契約上の名前で、概念名ではありません。
GitHub App JWT を外に出さないのは、GitHub App の権限全体を行使できる、Constitution のいう強い資格情報だからです（事実：Constitution の Design Principles 2）。

どの資格情報も、ログとエラー本文に記録しません（事実：Requirements QR-004）。
発行の判定に現れる資格情報は、種類によらず、どの出力でも値を伏せた同じ表記にします（推論：伏せ方が資格情報ごとに異なると、直すときに一部の資格情報だけが直り、ほかの資格情報が出力に残り得るため）。
一方、値を取り出せるコンポーネントは資格情報ごとに異なります（事実：Lambda Design §4）。
資格情報のうち利用者に返すのは Installation Access Token だけです（事実：Requirements FR-ISSUANCE-005）。

利用者接点には、これらに加えて **要求用トークン**（OIDC Request Token）が現れます。
Runner がジョブに渡す、GitHub OIDC Provider に OIDC JWT を要求するための資格情報です（事実：GitHub Actions Design §3）。
発行の判定には現れないため上の 5 種類に含めませんが、ログに出さず、値を取り出せるコンポーネントを限る点は同じです（事実：GitHub Actions Design §3.1、§3.4）。
利用者接点は、AWS 用 OIDC JWT と GATES 用 OIDC JWT を同じ手順で取得し、どちらになるかは要求した Audience で決まります。
そのため取得の手順では、両者をまとめて **OIDC JWT** と呼びます（事実：GitHub Actions Design §3.1）。

### Audit Record

**監査記録**（Audit Record）は、一回の発行処理の結果を、資格情報を含めずに残したものです（事実：Requirements FR-OPS-003）。

| 項目 | 記録する条件 | 根拠 |
|---|---|---|
| 相関 ID（Request ID） | 常に | 事実：Requirements FR-OPS-003 |
| 時刻、結果（成功、または失敗の種別を表すエラーコード） | 常に | 事実：Security Design §5 |
| 拒否の理由 | 失敗の種別で終わった場合。満たさなかった条件か失敗した呼び出しを示し、資格情報と Caller、要求、許可オーナー、保護対象の一覧、Installation の値を含めない。想定外の失敗では原因の値を含めない | 事実：Security Design §5、Lambda Design §3.2。保護対象の一覧の値を含めないのは推論：満たさなかった条件を示すのに要らず、文言が誤って応答に混ざっても、許可 Caller を含む一覧の内容を許可されない呼び出し元に明かさないため（Lambda Design §3.2 の許可オーナーと同じ理由） |
| Caller の Owner ID と Repository ID、ワークフロー実行 ID と試行番号 | 認証に成功した後だけ。認証前の失敗では未検証の値を Caller として記録しない | 事実：Security Design §5 |
| ワークフロー起動属性 | 認証に成功した後で、含まれていた属性だけ | 事実：Security Design §5 |
| 署名検証済みの Claims Set | 署名検証を通った後。署名検証の後の Claim の検証で認証に失敗した場合も含む | 事実：Requirements FR-OPS-003。署名検証の後の失敗を含めるのは推論：拒否した JWT の Claim が、拒否の理由を調べる手がかりになるため |
| 要求対象、要求権限 | 形式検証を通った後 | 事実：Lambda Design §1。時点は推論 |
| 取得した Installation の Installation ID とインストール先 Owner ID | Installation の解決に成功した後 | 推論：インストール先 Owner ID の不一致は運用者かオーナーの管理者が直す失敗であり、どのアカウントの Installation が返ったかを、呼び出しを再現せずに確かめるため。監査記録は応答に出ないため、エラー文言で値を伏せる理由は及ばない |
| 失効の結果 | 照合失敗で終わった場合 | 事実：Lambda Design §3.1 |

照合に失敗した発行処理では、失効を試みたかどうかと、試みた場合の成否を残します。
この値を **失効の結果**（Revocation Outcome）と呼び、試みなかった、試みて成功した、試みて失敗した、の 3 つのどれかをとります（語は候補）。
試みなかった場合を成否の一方に含めないのは、トークン文字列を取得できずに失効を試みなかった処理を、監査記録で失効を試みた処理と見分けるためです（推論：[INV-014 発行処理](#inv-014-発行処理)の不変条件は、トークン文字列を取得できた場合にだけ失効の試行を求めるため）。

常に記録する項目を除き、表の記録する条件は次の二つのどちらかです。
その値を確定する段階を発行処理が通ったか（ワークフロー起動属性は、そのうち含まれていた属性だけ）、その値を伴う結果で終わったか（拒否の理由は失敗、失効の結果は照合失敗）です。
監査記録は確定した値だけを持ち、確定していない項目を既定の値で表しません（推論：既定の値で表すと、認証前の失敗の記録に Caller があるように読めるため）。

監査記録の単位は発行処理 1 回です。
中断した発行処理では残らない場合があり、残っても結果が利用者の受け取った結果と一致しない場合があります（[Lifecycle of an Issuance](#lifecycle-of-an-issuance)）。
相関 ID は成功応答と失敗応答の両方に含め、ワークフローのログから監査記録へ、監査記録からワークフロー実行 ID でワークフロー実行へ、双方向にたどれるようにします（事実：Requirements FR-ISSUANCE-005、FR-OPS-003）。

### Concept Relationships

```mermaid
flowchart LR
  subgraph GitHub
    O[オーナー<br/>Owner]
    R[リポジトリ<br/>Repository]
    APP[GitHub App]
    INS[Installation]
    WF[ワークフロー実行<br/>Workflow Run]
  end
  subgraph GATES
    CFG[固定設定<br/>Config]
    C[Caller]
    REQ[発行要求<br/>Token Request]
    G[発行許可<br/>Token Grant]
    RES[発行結果<br/>Issuance Result]
    IT[返却可能な発行結果<br/>Returnable Issuance Result]
    AUD[監査記録<br/>Audit Record]
  end
  O -->|所有| R
  APP -->|オーナーごとに高々一つ| INS
  INS -->|インストール先| O
  INS -->|対象| R
  WF -->|実行元| R
  CFG -->|許可オーナーを固定| O
  CFG -->|利用 App を固定| APP
  CFG -->|保護対象を指定| R
  WF -->|GATES 用 OIDC JWT から確定| C
  WF -->|Docker Action 経由で送る| REQ
  C --> G
  REQ --> G
  CFG --> G
  INS -->|インストール先 Owner ID を照合| G
  G -->|GitHub API へ明示して発行| RES
  RES -->|発行許可と照合| IT
  C --> AUD
  REQ --> AUD
  IT -->|結果| AUD
```

## Behavior

### Operations

| 操作 | 英語名 | 担当 | 入力 | 結果 | 根拠 |
|---|---|---|---|---|---|
| トークンを要求する | （Docker Action の責務） | ワークフロー利用者 | 対象、権限、接続情報 | Docker Action が発行処理を起動する | 事実：Requirements FR-OPS-006 |
| ジョブの終了時の失効 | Revoke | Docker Action | ジョブの終了時の処理へ引き継いだ Installation Access Token。後続ステップへ渡したトークンはすべて含む | 成功または失敗。成否にかかわらずジョブの結果を変えない。トークンを引き継がなかった場合は何もしない | 事実：Requirements FR-ISSUANCE-006、GitHub Actions Design §3.5 |
| 失敗時の失効 | Revoke | Docker Action | 取り出した後に、Docker Action が失敗で終える（キャンセルで終える場合を含む）Installation Access Token | 成功または失敗。成否にかかわらず Docker Action は失敗で終わる | 事実：Requirements FR-ISSUANCE-006、GitHub Actions Design §3.5 |
| 形式検証 | Validate | Lambda Handler | Invoke Payload | 発行要求と未検証の GATES 用 OIDC JWT を取り出す、または入力不正 | 事実：Lambda Design §2.2 |
| 認証 | Authenticate | OIDC Verifier | 未検証の GATES 用 OIDC JWT、固定設定、Issuer の公開鍵、現在時刻 | Caller を確定する、または拒否 | 事実：Requirements FR-AUTHN-001 |
| 事前認可 | Authorize | Authorization | Caller、発行要求、許可オーナー、発行上限、保護対象の一覧 | 外部呼び出しに進める、または認可拒否 | 事実：Lambda Design §5 手順 3。保護対象の一覧は Requirements FR-AUTHZ-009 |
| 署名 | Sign | KMS Signer | 固定設定の Client ID と KMS Key ARN、現在時刻 | GitHub App JWT | 事実：Requirements FR-ISSUANCE-001、Lambda Design §4 |
| Installation の解決 | Resolve Installation | Token Issuer | GitHub App JWT、許可オーナーの名前と種別 | 取得した Installation、または拒否（一時的な障害を除く成功以外の応答、解釈できない応答、必須項目の欠落、停止状態を読み取れない応答、一時的な障害） | 事実：Lambda Design §3.2、§6.3 |
| 発行許可の確定 | Grant | Authorization | Caller、発行要求、許可オーナー、発行上限、保護対象の一覧、取得した Installation | 発行許可、または拒否（インストール先 Owner ID の不一致、停止、認可条件の不成立） | 事実：Lambda Design §5、Security Design §3.1。保護対象の一覧は Requirements FR-AUTHZ-009 |
| 発行 | Issue | Token Issuer | 発行許可、GitHub App JWT | 発行結果、または失敗 | 事実：Requirements FR-ISSUANCE-002、FR-ISSUANCE-003 |
| 発行結果の照合 | Verify | Token Issuer | 発行結果、発行許可、許可オーナー、保護対象の一覧、現在時刻 | 返却可能な発行結果、または拒否 | 事実：Requirements FR-ISSUANCE-004、FR-AUTHZ-010 |
| 失効 | Revoke | Token Issuer | 照合を通らなかった発行結果のトークン | 成功または失敗。成否にかかわらず拒否のまま終わる | 事実：Requirements FR-ISSUANCE-004、Lambda Design §3.1 |
| Installation Access Token の発行 | Issue Installation Access Token | Token Issuer | 発行許可の確定（Caller と発行要求は Token Issuer に渡さない）、許可オーナー、保護対象の一覧、現在時刻 | 署名、Installation の解決、発行許可の確定、発行、発行結果の照合を順に行い、返却可能な発行結果、またはいずれかの段階の拒否。照合失敗には失効の結果を添える | 事実：Lambda Design §6.2。保護対象の一覧は Requirements FR-AUTHZ-010。語は候補 |
| 監査記録の出力 | Audit | Lambda Handler | 発行処理の結果 | 監査記録 | 事実：Requirements FR-OPS-003 |
| 発行上限を変える | （運用手順） | GATES 運用者 | 新しい発行上限 | Lambda の設定の更新が完了した後の発行要求から、新しい発行上限で認可する。発行済みのトークンは失効させず、発行したときの権限のまま有効期限まで残る | 事実：Security Design §3.3 |
| 保護対象の一覧を変える | （運用手順） | GATES 運用者 | 新しい保護対象の一覧 | Lambda の設定の更新が完了した後の発行要求から、新しい一覧で認可し、照合する。発行済みのトークンは失効させず、発行したときの対象と権限のまま有効期限まで残る | 事実：Security Design §3.3、§3.6 |
| 署名鍵を更新する | （運用手順） | GATES 運用者 | 新しい Private Key | 鍵の集合への新しい KMS Key の追加と、使用中の鍵（Lambda が参照する Key ARN）の切り替え | 事実：Requirements FR-OPS-005、AWS Infrastructure Design §4.3 |
| Installation の対象を変える | （GitHub 側の操作） | GitHub App / オーナーの管理者 | 対象リポジトリの選択 | 読み取り可能範囲の上限が変わる | 事実：Security Design §3.2 |

操作の失敗は、[Failure Kinds](#failure-kinds) の種別に次のように対応します（事実：Lambda Design §3.2、§5）。

| 操作 | 失敗の種別 |
|---|---|
| Validate | 入力不正 |
| Authenticate | 認証失敗。Discovery / JWKS を一時的に取得できなければ一時的な障害、GitHub OIDC Provider の応答が想定と異なれば内部障害 |
| Authorize | 認可拒否 |
| Sign | 一時的な障害。署名の要求を送る前に失敗するか、KMS が再試行では解消しない失敗を返すか、KMS の応答を署名として解釈できなければ内部障害 |
| Resolve Installation | Installation 利用不可（App JWT の拒否や未インストールを含む、一時的な障害を除く成功以外の応答。応答を解釈できない、必須項目が欠落している、停止状態を読み取れない）。取得時の一時的な障害は一時的な障害 |
| Grant | Installation 利用不可（インストール先 Owner ID の不一致、停止）。認可条件を満たさなければ認可拒否（Authorize を通った要求では起きないため、[Lifecycle of an Issuance](#lifecycle-of-an-issuance) の図には描かない） |
| Issue | GitHub 発行拒否（発行 API の `422`）。通信の失敗などは一時的な障害、その他の失敗は Installation 利用不可 |
| Verify | 照合失敗 |
| Revoke | なし。失効に失敗しても照合失敗のまま終わる |
| Issue Installation Access Token | 失敗した段階（Sign、Resolve Installation、Grant、Issue、Verify）の種別をそのまま返す |

どの段階にも属さない想定外の障害も、Handler が応答を書ける限り内部障害で終わります。

運用手順と GitHub 側の操作は発行処理の外で行われますが、発行の結果か発行できる範囲を変えるため、モデルに含めます。
認可の境界は GATES の設定（許可オーナー、発行上限、保護対象の一覧）だけでは決まらず、Installation の対象と GitHub App に付与された権限に依存します。

### Failure Kinds

発行処理がトークンを返さずに終わる失敗を、**失敗の種別**（Failure Kind）で区別します。
種別を分けるのは、区別がルール、監査記録の意味、失効の義務のいずれかを変える場合に限ります（推論：本ドキュメントが答える問いが判定のルールであるため）。
どの外部依存が失敗したかのように、直す担当も次に起きることも変えない区別は、応答のエラーコードに任せます。

| 種別 | 英語名 | 起きる条件 | 直す担当と次に起きること | 根拠 |
|---|---|---|---|---|
| **入力不正** | Invalid Request | Payload を GATES が読み取れない | 利用者が要求の形を直す。要求を読み取れていないため、監査記録に要求対象と要求権限を残さない | 事実：Lambda Design §2.2 |
| **認証失敗** | Authentication Failure | GATES 用 OIDC JWT から Caller を確定できない | 利用者がワークフローの設定を直す。OIDC JWT の主体のカスタマイズが原因なら、リポジトリかオーナーの管理者が直す（[Events Outside GATES That Change the Outcome](#events-outside-gates-that-change-the-outcome)）。Issuer の公開鍵のローテーションの直後なら、時間をおいた再実行で解消し得る（同節）。監査記録に未検証の値を Caller として残さない | 事実：Requirements FR-AUTHN-001、Security Design §5 |
| **認可拒否** | Authorization Denied | Caller と要求は読み取れたが、GATES の認可条件を満たさない。保護対象に当たる対象を含む要求で、Caller が許可 Caller に当たらないか、Permissions が許可 Caller の権限の上限に水準の包含で含まれない場合を含む | 利用者が要求の対象か権限を直す。保護対象の一覧が GitHub の現在の状態と食い違っている場合（許可 Caller のリポジトリの改名など）は、GATES 運用者が一覧を直す。監査記録には、GATES の認可で拒否したことと、満たさなかった条件が残る | 事実：Security Design §3.1、§3.6 |
| **Installation 利用不可** | Installation Unavailable | 固定した GitHub App と許可オーナーから、発行に使える Installation を得られない。一時的な障害を除く成功以外の応答、応答を解釈できない、必須項目の欠落、停止状態を読み取れない、停止、インストール先 Owner ID の不一致、発行の間に Installation か App の状態が変わった場合 | GATES 運用者かオーナーの管理者が、GitHub 側か固定設定を直す。再試行では解消しない | 事実：Lambda Design §3.2 |
| **GitHub 発行拒否** | GitHub Issuance Rejected | GATES の認可が成立したが、GitHub 発行 API が `422` で要求を拒否した。原因は応答コードから確定できない | 利用者の許可外要求とは断定せず、運用者が GitHub の発行条件・状態・応答を調べる。トークン文字列は未取得 | 事実：Lambda Design §3.2 |
| **照合失敗** | Verification Failure | GitHub 発行 API の応答を取得したが、発行結果が発行許可と一致しないか、実効対象のリポジトリ名と Repository ID の対応が保護対象の一覧と食い違う。発行結果に読み取れなかった項目がある場合を含む | 運用者が設定か GitHub 側の仕様変更を調べる。保護対象の一覧との食い違いなら、GATES 運用者が一覧を GitHub の現在の状態に合わせて直す。トークン文字列を取得できた場合は失効を試みる | 事実：Requirements FR-ISSUANCE-004、Lambda Design §3.1 |
| **一時的な障害** | Transient Failure | Discovery / JWKS の取得、KMS の署名、GitHub API の呼び出しのいずれかが一時的に失敗した。通信の失敗、タイムアウト、サーバー側のエラー、レート制限を指す | 利用者の再実行で解消し得る。GATES も Docker Action も要求を自動で送り直さない（[Contexts](#contexts)） | 事実：Lambda Design §3.2、§6.4、GitHub Actions Design §3.3 |
| **内部障害** | Internal Failure | 想定外の失敗。どの段階にも属さない失敗と、GitHub OIDC Provider の応答が想定と異なる失敗（Discovery が示す Issuer が固定した Issuer と異なる、公開鍵の取得先が Issuer と異なるホストかポートを指す、応答を解釈できない、公開鍵集合に秘密鍵か対称鍵の値が含まれる、使用可能な鍵が一つもない、応答が上限より大きい、再試行では解消しない失敗の応答が返る）と、署名の要求を送る前に失敗した場合、KMS が再試行では解消しない失敗を返した場合（鍵の無効化、署名できない鍵の状態、権限の不足など）、KMS の応答を署名として解釈できない場合を含む | 運用者が調べる。再試行では解消しない | 事実：Lambda Design §3.2 |

Installation 利用不可を認可拒否と分けるのは、利用者の要求に起因しない失敗だからです。
認可拒否に含めると、監査記録では利用者の要求が許可外だったように見えます。
一時的な障害と分けるのは、再試行では解消せず、運用者の対応を要するためです。

Installation 利用不可と照合失敗は、どちらも設定ミスや GitHub 側の仕様変更を示し得る監視対象です。
両者の境界は、問題を検出した処理段階です。Installation の取得・確認時点の不整合は Installation 利用不可、発行応答を取得して照合した不整合は照合失敗です。
照合失敗を一時的な障害と分けるのも、発行結果の不整合が再試行で解消する障害ではなく、監視対象だからです。

一時的な障害は、失敗した依存先が Discovery / JWKS、KMS、GitHub API のどれであっても一つの種別にまとめます。
どれが失敗しても、直す担当は変わらず、次に起きることも再試行で変わらないためです。
依存先の区別は調査のための情報であり、応答のエラーコードで表します。

GitHub OIDC Provider の応答が想定と異なる失敗を一時的な障害と分けるのは、再試行では解消せず、一時的な障害にすると利用者に無駄な再実行を促すためです。
新しい種別を設けず内部障害に含めるのは、内部障害と比べて、直す担当（運用者）も、監査記録に残るもの（失敗した段階までに確定した値）も、失効の義務（発行前のため、なし）も変わらないためです。
原因は監査記録の拒否の理由で区別します。
署名の要求を送る前に失敗した場合、KMS が再試行では解消しない失敗を返した場合、KMS の応答を署名として解釈できない場合を内部障害に含める理由も同じです。
鍵の状態、Key Policy、固定設定、実行環境を運用者が直すまで解消せず、発行の前のため失効の義務もありません。

操作が返して発行処理を終わらせる失敗は、ちょうど一つの種別に属し、その種別は操作が定めます（推論：複数の種別に属すると、直す担当と次に起きることが一つに決まらないため）。

失敗の種別は、応答のエラーコードで表します。
種別とエラーコードは一時的な障害を除いて一対一に対応し、一時的な障害だけは失敗した依存先ごとにエラーコードを分けます。

### Lifecycle of an Issuance

一回の発行処理は、次の状態を順にたどります。
各段階で失敗すると、その段階に対応する失敗の種別で終わり、以降の段階には進みません（事実：Requirements QR-003、Lambda Design §5）。

```mermaid
stateDiagram-v2
  state "受理" as received
  state "形式検証済み" as validated
  state "認証済み" as authenticated
  state "事前認可済み" as preauthorized
  state "署名済み" as signed
  state "Installation 解決済み" as resolved
  state "発行許可済み" as granted
  state "発行 API 応答取得済み" as issue_responded
  state "発行 API 呼出中" as issue_requested
  state "返却可能" as returnable
  state "入力不正" as rej_request
  state "認証失敗" as rej_auth
  state "認可拒否" as rej_authz
  state "Installation 利用不可" as rej_installation
  state "GitHub 発行拒否" as rej_issue
  state "照合失敗" as rej_verify
  state "一時的な障害" as rej_transient
  state "内部障害" as rej_internal
  [*] --> received
  received --> validated : 形式検証
  received --> rej_request : 入力不正
  validated --> authenticated : 認証（Caller 確定）
  validated --> rej_auth : JWT 不正
  validated --> rej_transient : Discovery / JWKS の一時的な取得不能
  validated --> rej_internal : GitHub OIDC Provider の応答が想定と異なる
  authenticated --> preauthorized : 事前認可
  authenticated --> rej_authz : Owner ID、オーナー名、権限の不一致、保護対象の条件の不成立
  preauthorized --> signed : 署名
  preauthorized --> rej_transient : KMS の一時的な失敗
  preauthorized --> rej_internal : 要求を送る前の失敗、KMS の再試行では解消しない失敗、署名として解釈できない応答
  signed --> resolved : Installation の解決
  signed --> rej_installation : 一時的な障害を除く成功以外の応答（App JWT の拒否、未インストールなど）、解釈できない応答、必須項目の欠落、停止状態を読み取れない応答
  signed --> rej_transient : 取得時の一時的な障害
  resolved --> granted : 発行許可の確定
  resolved --> rej_installation : 停止、インストール先 Owner ID の不一致
  granted --> issue_requested : 発行 API 呼出し
  issue_requested --> issue_responded : 発行応答を受信
  issue_requested --> rej_issue : 発行 API が要求を拒否
  issue_requested --> rej_transient : 応答喪失を含む一時的な障害（発行有無は不明）
  issue_requested --> rej_installation : その他の失敗
  issue_responded --> returnable : 発行結果の照合
  issue_responded --> rej_verify : 発行許可か保護対象の一覧との不一致（取得済みトークンを失効試行）
  returnable --> [*]
```

内部障害への遷移は認証と署名の段階のものだけを描き、どの段階でも起き得る想定外の失敗は描きません。

照合失敗は GitHub の発行応答を取得した後の拒否です。空でないトークン文字列を取得した場合は失効を試みます。発行 API への要求後に通信が断たれた場合は、GitHub 側で発行が成功していてもトークン文字列を取得できないことがあります。返却しないことは保証しますが、GitHub 側でトークンが残らないことは保証しません（事実：Requirements FR-ISSUANCE-004、QR-003）。

始まった発行処理は、失敗の種別で終わらずに打ち切られることもあります。
タイムアウトやメモリ不足で Handler が応答を書けない場合で、これを **中断**（Interruption）と呼びます（事実：Lambda Design §3.2。語は候補）。
中断を失敗の種別に含めないのは、種別は発行処理が判定して応答で表すものであり、中断では応答で表せないためです（推論）。
タイムアウトでは、発行処理が期限切れを失敗の種別に判定した後、応答を返す前に打ち切られることがあります（事実：[AWS Fault Injection](../research/aws/fault-injection.md)）。
中断はどの段階でも起き得るため、状態図には描きません。
中断した発行処理は利用者にトークンを返しませんが、監査記録が残らない場合があります。
残る場合もあり、その結果は発行処理が判定した結果（成功または失敗の種別）で、利用者が受け取った結果（応答のない中断）とは一致しません（推論：Lambda Handler は、成功でも失敗でも監査記録を書いてから応答を返すため）。
トークン文字列を取得した後に中断すると、失効を試みられず、GitHub 側でトークンが有効なまま残り得ます（推論：Handler が処理を続けられないため）。

Handler が応答を書けない Invoke には、発行処理を始めないものもあります。
初期化の失敗と、相関 ID が UTF-8 として正しくない場合です（事実：Lambda Design §3.2）。
初期化の失敗は、固定設定が不変条件（[INV-004 許可オーナー](#inv-004-許可オーナー)、[INV-005 認証の固定設定](#inv-005-認証の固定設定)、[INV-006 署名の固定設定](#inv-006-署名の固定設定)、[INV-015 発行上限](#inv-015-発行上限)、[INV-016 保護対象の一覧](#inv-016-保護対象の一覧)）を満たさないか、実行環境から AWS SDK の設定を読めず、部品を作れない場合で、その実行環境は受け取った Invoke に対して発行処理を始めません（推論：不変条件を満たさない値を作らない方針から）。
どちらも中断と異なり、監査記録は残らず、GitHub 側にトークンを残すこともありません（推論：発行処理を始める前に止まり、GitHub API を呼ばないため）。

認可が事前認可と発行許可の確定の二段に分かれるのは、Installation を取得しないとインストール先 Owner ID を照合できず、Installation の取得には署名済みの GitHub App JWT が要るためです（推論：Lambda Design §5 手順 3 から 5 の順序から）。
事前認可は、KMS と GitHub API を呼ぶ前に、認可の見込みがない要求を落とすための段階です（事実：Lambda Design §5）。

### Events Outside GATES That Change the Outcome

発行処理の外で起きる次の出来事は、GATES の設定を変えなくても発行の結果を変えます。

| 出来事 | 影響 | 根拠 |
|---|---|---|
| オーナーの名前変更 | IAM Role の引き受けとオーナー名の照合が失敗し、設定を更新するまで発行が止まる | 事実：AWS Infrastructure Design §3.1 |
| リポジトリの名前変更 | 旧名で要求すると、GitHub が `422` で拒否し、GitHub 発行拒否になる。GitHub が新しい名前のリポジトリに読み替えて発行した場合も、実効対象の完全名が一致せず返却しない。同一の許可オーナー・Installation の対象に残り、ほかの条件も成立するなら、新しい完全名で要求できる。保護対象と許可 Caller のリポジトリの改名は、それぞれの行に示す | 事実：GitHub App Constraints §4.2 の観測（旧名への `422`。GitHub は保証していない）。読み替えた場合は推論：Security Design §3.4 の照合条件から |
| 保護対象の名前変更 | 新しい名前で要求すると、発行前は保護対象に当たらずに通り、GitHub 側でトークンが発行される。実効対象の Repository ID が保護対象の ID と一致し、リポジトリ名が一致しないため、照合失敗になり、失効を試みる。旧名で要求すると、発行前は保護対象として判定され、認可拒否か、認可が成立しても GitHub が `422` で拒否して GitHub 発行拒否になる。どちらも、運用者が一覧の名前を直すまで続く | 推論：INV-012 と RULE-015 から。旧名への `422` は事実：GitHub App Constraints §4.2 の観測（GitHub は保証していない） |
| 保護対象の削除と、同じ名前での作り直し | 作り直したリポジトリは別の Repository ID を持つ。その名前で要求すると、発行前は保護対象として判定され、Caller が許可 Caller に当たらなければ認可拒否、当たれば発行の後に、リポジトリ名が一致して Repository ID が一致しないため照合失敗になる。作り直したリポジトリに、前の保護対象の保護も許可も引き継がない | 推論：INV-012 と RULE-015 から。作り直しで ID が変わることは事実：GitHub Owner and Repository Constraints §3 の観測 |
| 許可 Caller のリポジトリの名前変更 | Repository ID は一致し、リポジトリ名が一致しないため、許可 Caller に当たらず認可拒否になる。運用者が一覧の名前を直すまで、そのリポジトリのワークフローは保護対象を要求できない | 推論：RULE-015 から。改名で ID が変わらないことは事実：GitHub Actions OIDC Constraints の GH-OIDC-REPOSITORY-ID-RENAME。改名の後の JWT が新しい名前を示すことは同 GH-OIDC-REPOSITORY-NAME-OBSERVED の観測（GitHub は保証していない） |
| 許可 Caller のリポジトリの削除と、同じ名前での作り直し | リポジトリ名は一致し、Repository ID が一致しないため、許可 Caller に当たらず認可拒否になる。作り直したリポジトリに許可を引き継がない | 推論：RULE-015 から。作り直しで ID が変わることは事実：GitHub Owner and Repository Constraints §3 の観測 |
| リポジトリの移管 | 所有オーナーと Installation の対象範囲が変わり得る。異なるオーナーへの移管後は、名前を変えて要求するだけでは単一オーナー認可を満たさない | 推論：Requirements FR-AUTHZ-001・FR-AUTHZ-005 から |
| Installation の対象の追加・削除 | 読み取り可能範囲の上限が変わる。Organization では、制限していなければリポジトリ管理者も追加できる | 事実：Security Design §3.2 |
| GitHub App の権限の変更 | 広げても、許可権限に含まれないか、発行上限に水準の包含で含まれない権限を含む要求は、認可拒否のまま変わらない。保護対象への要求では、許可 Caller の権限の上限に水準の包含で含まれない権限も同じ。狭めると、付与されなくなった権限を含む要求は GitHub 発行拒否か照合失敗になる | 事実：Security Design §3.3、Requirements FR-AUTHZ-007 |
| GitHub App の再インストール | Installation ID が変わるが、設定変更なしに新しい Installation を導出する | 事実：Security Design §3.2 |
| Installation の停止 | 取得した Installation が停止状態のため、発行許可を作らず Installation 利用不可で閉じる。GitHub の発行 API を呼ばない | 事実：Security Design §3.1 |
| GitHub 側での旧 Private Key の失効 | 新しい KMS Key に切り替え済みなら影響しない | 事実：Acceptance Criteria AC-ISSUANCE-005 |
| OIDC JWT の主体のカスタマイズ | GATES が受け付ける主体の形式から外れると、入口制御の拒否か認証失敗で発行が止まる | 推論：GATES OIDC JWT Claims の GOJ-SUB-FORMAT、AWS Infrastructure Design §3.1 の主体の条件から。実環境では未確認 |
| Issuer の公開鍵のローテーション | 手元の鍵で検証できない JWT で JWKS の再取得が許され、取得に成功すれば新しい鍵に追従できる。再取得の上限の間や取得失敗時は、正当な JWT でも一時的に拒否され得る | 推論：Lambda Design §6.1 の再取得の上限と検証条件から |
| 公開鍵の取得先の変更 | 保持した Discovery は取り直さないため、古い取得先が使えなくなると、実行環境が入れ替わるまで一時的な障害か内部障害で止まる | 推論：Lambda Design §6.1 の Discovery の再利用から |

## Rules and Invariants

### Identity Rules

#### RULE-001 ID による同定

**内容**：オーナーの同一性は不変 Owner ID で判断し、名前の一致では代用しない

**根拠**：事実：Requirements FR-AUTHZ-001

#### RULE-002 ID の形式

**内容**：Owner ID、Repository ID、Installation ID、ワークフロー実行 ID、試行番号は、int64 に収まる正の整数である。Owner ID、Repository ID、Installation ID は互いに別の概念で、比べるのは同じ概念の ID どうしに限る

**根拠**：推論：GitHub の ID は 1 から始まる。試行番号が初回に `1` になることは未確認で、`0` を拒否するのは GATES の判断

#### RULE-003 名前の照合は補助

**内容**：完全名のオーナー部分と許可オーナー名の照合は、要求の整合性の確認であり、認可の根拠ではない。認可の根拠は Caller の Owner ID にあり、発行に使う Installation はインストール先 Owner ID で確かめる。保護対象と許可 Caller の同一性も Repository ID で判断する。保護対象のリポジトリ名は、発行前に保護対象に当たる対象を見分ける手がかりで、その判定を、発行結果の照合で実効対象の Repository ID と突き合わせる。許可 Caller のリポジトリ名は、Repository ID の一致に重ねて求める条件で、一覧の誤りを拒否に倒すためのものであり、名前の一致だけで許可 Caller に当てない

**根拠**：推論：Security Design §3.1 の条件 1 と Installation の確認が ID、条件 2 だけが名前であることから。保護対象と許可 Caller は事実：Requirements FR-AUTHZ-009、FR-AUTHZ-010

#### RULE-004 ID の整合

**内容**：Caller の Owner ID と Repository ID は、GATES 用 OIDC JWT の中で主体を表す属性と個別の属性の両方に現れる。両方がそろい、互いに一致するときに限り Caller を確定する

**根拠**：事実：GitHub Actions OIDC Constraints、GATES OIDC JWT Claims の Cross-Claim Invariants

#### RULE-005 名前の比較規則

**内容**：完全名とリポジトリ名の比較と重複判定は、ASCII の範囲で大文字小文字を区別しない。要求の重複判定、オーナー名の照合、実効対象の完全名の照合、保護対象のリポジトリ名と要求のリポジトリ名の照合、許可 Caller のリポジトリ名と Caller のリポジトリ名の照合、保護対象の一覧の重複判定で共通

**根拠**：事実：Lambda Design §2.2。リポジトリ名への適用は推論：GitHub の REST API がパスのリポジトリ名の大文字小文字を区別しないため（GitHub Owner and Repository Constraints §1）

### Rules

全件成立（Requirements FR-AUTHZ-004、FR-AUTHZ-007）、有効期限を延長しないこと（FR-ISSUANCE-002）、発行 API の呼び出しでの対象と権限の明示（FR-ISSUANCE-003）、付随権限 `metadata:read` の許容（FR-ISSUANCE-004）、Fail Closed（QR-003）、資格情報の非記録（QR-004）は、要求をそのまま発行処理の規則とします。
本節では定義し直さず、要求の ID で参照します。

#### RULE-006 認証条件

**適用対象**：認証

**成立条件**：GATES 用 OIDC JWT が、JWS Compact Serialization で表され、JOSE Header が未対応の拡張や Nested JWT を示さず、`kid` で一意に選べる Issuer の使用可能な鍵で GATES が許可した署名方式により署名され、Issuer が固定設定の Issuer と一致し、Audience が GATES 用 Audience の一つだけであり、現在時刻が失効時刻より前であり、有効期間の開始と発行時刻が現在時刻に許容時計差を足した時刻より後でなく、属性を一意に読み取れ、発行時刻、失効時刻、Caller の確定に要る属性がそろい、[RULE-004 ID の整合](#rule-004-id-の整合)を満たす。一つでも満たさなければ認証失敗

**例外**：有効期間の開始は欠落を許す

**根拠**：事実：Requirements FR-AUTHN-001、FR-AUTHN-002、GATES OIDC JWT Claims、GATES OIDC JWT Signature

#### RULE-007 取得前の拒否

**適用対象**：認証

**成立条件**：直列化、JSON と Claim 名、NumericDate の型と範囲、JOSE Header の条件（`kid` の存在を含む）は Issuer の公開鍵に依存しないため、公開鍵集合を取得する前に判定する。満たさなければ取得せずに認証失敗とし、再取得の上限を消費しない

**例外**：なし

**根拠**：事実：Lambda Design §6.1

#### RULE-008 鍵の選択と再取得

**適用対象**：認証

**成立条件**：保持した集合から `kid` で一意に選べる使用可能な鍵で署名を検証する。選べない（未知の `kid`、除外した JWK だけに一致、使用可能な鍵の複数一致）か署名が一致しなければ、再取得の上限の範囲で集合を取り直して判定し直し、それでも満たさなければ認証失敗。複数の鍵を総当たりで試さない

**例外**：上限の間は取り直さず、前回の取得が失敗していれば前回の失敗の種別を返す

**根拠**：事実：Lambda Design §6.1、GATES OIDC JWT Signature の GOJ-JWKS-KID、GOJ-SIGNATURE

#### RULE-009 JWK の除外

**適用対象**：公開鍵集合の取得

**成立条件**：受理条件を満たさない JWK は、その JWK だけを除外して記録し、残りの使用可能な鍵で集合を作る。秘密鍵か対称鍵の値を含む JWK が一つでもあれば、`kty` や JWK として読めるかを問わず、集合全体を受け入れず内部障害

**例外**：なし

**根拠**：事実：GATES OIDC JWT Signature の GOJ-JWKS-USABLE、GOJ-JWKS-SECRET

#### RULE-010 認可条件

**適用対象**：事前認可、発行許可の確定

**成立条件**：Caller の Owner ID が許可オーナーの Owner ID と一致し、要求の各完全名のオーナー名が許可オーナー名と一致し、Permissions の Permission Entry がどれも許可権限に含まれ、発行上限に[水準の包含](#permission-levels)で含まれる。満たさなければ認可拒否。許可権限か発行上限に含まれない Permission Entry を除いたり、水準を変えたりして成立させない

**例外**：なし

**根拠**：事実：Security Design §3.1、§3.3、Requirements FR-AUTHZ-008。水準の包含は許可権限に含まれることを条件に含み、発行上限は [INV-015 発行上限](#inv-015-発行上限)により許可権限の一部だが、許可権限の判定も独立した条件として残す（推論：許可権限を、固定設定の読み込みの誤りにも水準の比較の誤りにも左右されない最後の上限として保つため）

#### RULE-015 保護対象の認可条件

**適用対象**：事前認可、発行許可の確定

**成立条件**：要求の完全名のうち、リポジトリ名が保護対象のリポジトリ名と一致するものを、その保護対象に当たる対象とする。保護対象に当たる対象のすべてについて、その保護対象の許可 Caller のうち、Repository ID が Caller の Repository ID と一致し、かつリポジトリ名が Caller のリポジトリ名と一致するものがあり、要求の Permission Entry がどれもその許可 Caller の権限の上限に水準の包含で含まれる。満たさない保護対象が一つでもあれば、保護対象に当たらない対象を含めて要求全体を認可拒否とする。対象か Permission Entry を除いて成立させない。許可 Caller のない保護対象に当たる対象を含む要求は、Caller によらず条件を満たさない

**例外**：保護対象に当たる対象を含まない要求には、条件を課さない

**根拠**：事実：Requirements FR-AUTHZ-009、FR-AUTHZ-010、FR-AUTHZ-004。名前の比較は [RULE-005 名前の比較規則](#rule-005-名前の比較規則)。[RULE-010 認可条件](#rule-010-認可条件)に含めず別の規則にするのは推論：RULE-010 の条件は、許可オーナー、GATES のコードが固定する許可権限、デプロイに一つの発行上限で決まり、Caller と対象の組を見ない。この規則は、GATES 運用者が保護対象ごとに Caller と権限を決め、リポジトリの改名や作り直しを契機に変わる。決める単位と変わる契機が異なるため、別に判定し、別に変えられるようにする

発行前の判定はリポジトリ名だけで保護対象を見分けるため、この規則だけでは、改名した保護対象を新しい名前で要求した場合と、保護対象の名前を持つ別のリポジトリを要求した場合を正しく扱えません。
その発行結果は、[INV-012 返却可能な発行結果](#inv-012-返却可能な発行結果)の、実効対象のリポジトリ名と Repository ID の対応の条件で返しません。
二つは組で働き、どちらか一方では保護になりません（推論）。

#### RULE-011 Installation の有効性

**適用対象**：発行許可の確定

**成立条件**：取得した Installation のインストール先 Owner ID が許可オーナーの Owner ID と一致し、停止していない。満たさなければ Installation 利用不可

**例外**：なし

**根拠**：事実：Security Design §3.1

Installation に対象リポジトリが含まれることを、GATES は発行前に確認しません（事実：Security Design §2 の Authorization の行）。
含まれない対象は、発行 API が呼び出しを受け付けないか、発行結果の対象一覧が要求と一致しないかのどちらかで検出します（推論）。
失敗の種別は GitHub の応答で決まり、発行 API の `422` なら GitHub 発行拒否、その他の失敗なら Installation 利用不可、発行応答が発行許可と一致しなければ照合失敗です。
種別からは、対象が Installation に含まれなかったことを断定できません。
どの場合も利用者にトークンを返しません。

#### RULE-012 対象の上限

**適用対象**：発行要求

**成立条件**：対象は 500 件以下

**例外**：なし

**根拠**：事実：Lambda Design §2.1、GitHub App Constraints

#### RULE-013 入力不正と認可拒否の区別

**適用対象**：形式検証と認可

**成立条件**：値を読み取れない形は入力不正、読み取れて許可条件に合わない値は認可拒否

**例外**：なし

**根拠**：事実：Lambda Design §2.2

#### RULE-014 設定の固定

**適用対象**：固定設定

**成立条件**：実行時の入力から許可オーナー、GitHub App、Installation、KMS Key、Issuer、JWKS、Audience、API Host、発行上限、保護対象の一覧を指定できない

**例外**：なし

**根拠**：事実：Requirements FR-AUTHZ-002、FR-OPS-001

### Invariants

不変条件は、その値を作る時点で条件を検査し、条件を満たさない値を作らないことで守ります。

#### INV-001 発行要求

**時点**：形式検証を通った時点

**常に成り立つこと**：対象が 1 件以上 500 件以下で、各要素が完全名の形式を満たし、大文字小文字を区別しない重複がない。Permissions が [INV-002 Permissions](#inv-002-permissions) の条件を満たす

**根拠**：事実：Lambda Design §2.2

#### INV-002 Permissions

**時点**：作られた時点

**常に成り立つこと**：Permission Entry を 1 つ以上持ち、権限名と水準がどれも権限名と水準の文字種を満たす。Permission Entry が許可権限に含まれるかは含まず、事前認可が判定する

**根拠**：事実：Lambda Design §2.2

#### INV-003 相関 ID

**時点**：Lambda Handler に渡す値を作った時点

**常に成り立つこと**：UTF-8 として正しい文字列である

**根拠**：事実：Lambda Design §3.2

#### INV-004 許可オーナー

**時点**：固定設定を読んだ時点

**常に成り立つこと**：Owner ID が [RULE-002 ID の形式](#rule-002-id-の形式)を満たし、オーナー名がオーナー名の文字種を満たし、種別が二つの値のどちらかである

**根拠**：事実：Requirements FR-OPS-001、Lambda Design §2.2 の名前の文字種。推論：条件を満たさない値を作らない方針（[Invariants](#invariants) の冒頭）

#### INV-005 認証の固定設定

**時点**：OIDC Verifier を作った時点

**常に成り立つこと**：Issuer が `https` のホストと、あれば空でないポートだけからなり、解析しても文字列が変わらない URL で、Audience が空でなく、AWS 用 OIDC JWT の Audience `sts.amazonaws.com` と異なり、ASCII の範囲で大文字小文字を区別せずに比べて、`https://github.com` と異なり、`https://github.com/` で始まらない

**根拠**：推論：空の Audience はすべての JWT を拒否し、空の Issuer は Issuer の値のない署名済み JWT を通す。`sts.amazonaws.com` を拒否するのは、同じジョブで取得する AWS 用 OIDC JWT で認証を通せるようにしないため。github.com の URL を拒否するのは、GitHub が GitHub 全体を指す `https://github.com` と、呼び出し元のオーナーとリポジトリの URL でも発行し（事実：GitHub Actions OIDC Constraints）、オーナーの URL は Audience を指定しない OIDC JWT にも入るため（事実：GATES OIDC JWT Claims の GOJ-AUD）、同じ値を Audience に使うほかのサービス向けの OIDC JWT や、Audience を指定せずに取得した OIDC JWT で認証を通せるようにしないため。前方一致で拒否するのは、発行される URL が呼び出し元ごとに異なり、値を列挙できないため。大文字小文字を区別しないのは、ホスト名、オーナー名、リポジトリ名が大文字小文字を区別しないため。RFC 3986 §6.2.3 が同じ URL とする、空のポートや既定のポートを付けた形は、Audience に選ばれる表記と考えにくいため扱わない。`https` に限るのは、平文の通信を設定から有効にできる形にしないため。ポートを許すのは、`httptest` の Issuer で取得と失敗の分類を確かめるため

#### INV-006 署名の固定設定

**時点**：KMS Signer を作った時点

**常に成り立つこと**：Client ID が空でなく、UTF-8 として正しい。KMS Key ARN が空でない

**根拠**：推論：UTF-8 として正しくない Client ID は GitHub App JWT の JSON にできず、空の Client ID の GitHub App JWT は GitHub に拒否され、空の KMS Key ARN では KMS が署名しない。どれも発行のたびに失敗する設定であり、作る時点で止める

#### INV-015 発行上限

**時点**：固定設定を読んだ時点

**常に成り立つこと**：Permission Entry を 1 つ以上持ち、どの Permission Entry も許可権限に含まれる

**根拠**：事実：Requirements FR-AUTHZ-008。推論：条件を満たさない値を作らない方針（[Invariants](#invariants) の冒頭）

#### INV-016 保護対象の一覧

**時点**：固定設定を読んだ時点

**常に成り立つこと**：保護対象の一覧と、各保護対象の許可 Caller の集合は、空でもよい。保護対象と許可 Caller の Repository ID は [RULE-002 ID の形式](#rule-002-id-の形式)を満たし、リポジトリ名はリポジトリ名の文字種を満たす。保護対象の間で Repository ID とリポジトリ名のどちらも重複しない。同じ保護対象の許可 Caller の間で Repository ID とリポジトリ名のどちらも重複しない。許可 Caller の権限の上限は Permission Entry を 1 つ以上持ち、権限名が重複せず、どの Permission Entry も許可権限に含まれる。権限の上限が発行上限に含まれることは求めない

**根拠**：事実：Requirements FR-OPS-001、FR-AUTHZ-009。文字種は Lambda Design §2.2。重複の判定は [RULE-005 名前の比較規則](#rule-005-名前の比較規則)。推論：文字種を一覧に課す理由は [リポジトリ名](#github-entities-referenced-by-gates)の根拠。保護対象のリポジトリ名が重複すると、発行前に一つの対象が当たる保護対象が一つに決まらない。保護対象の Repository ID の重複は、同じリポジトリに二つの名前が対応し、どちらかの名前の要求が必ず照合失敗になる書き誤りである。同じ保護対象の許可 Caller が重複すると、権限の上限が一つに決まらない。空の権限の上限は、許可 Caller を置かないことと区別できない書き誤りである。発行上限に含まれることを求めない理由は [Protected Repositories](#protected-repositories)。条件を満たさない値を作らない方針（[Invariants](#invariants) の冒頭）

#### INV-007 保持した Discovery

**時点**：保持した時点

**常に成り立つこと**：Issuer が固定設定の Issuer と一致し、公開鍵の取得先が Issuer と同じ `https` のホストとポートを指す

**根拠**：事実：OpenID Connect Discovery、GATES OIDC JWT Signature の GOJ-DISCOVERY-ISSUER、GOJ-DISCOVERY-JWKS-URI。ホストとポートの限定は推論：固定した Issuer の外から鍵を読まないため

#### INV-008 保持した公開鍵

**時点**：保持した時点

**常に成り立つこと**：完全に受信して解釈でき、秘密鍵か対称鍵の値を含まない固定 Issuer の応答の、使用可能な鍵だけからなり、1 つ以上ある。除外した JWK は `kid` を含めて保持しない。同じ `kid` の使用可能な鍵が複数あっても保持し、一意に選べるかは JWT ごとに判定する。取得に失敗しても前回の鍵が残り、未知の鍵で検証に成功したとは扱わない

**根拠**：事実：GATES OIDC JWT Signature の GOJ-JWKS-SECRET、GOJ-JWKS-USABLE、GOJ-JWKS-KID。保持・更新の方式は Lambda Design §6.1

#### INV-009 Caller

**時点**：認証を通った時点

**常に成り立つこと**：[RULE-006 認証条件](#rule-006-認証条件)を満たした GATES 用 OIDC JWT だけから作られ、Owner ID と Repository ID が [RULE-004 ID の整合](#rule-004-id-の整合)を満たす。Owner ID、Repository ID、ワークフロー実行 ID、試行番号はどれも [RULE-002 ID の形式](#rule-002-id-の形式)を満たす。リポジトリ名は、主体を表す属性のうち Repository ID と組になった名前で、空でない。いずれかが欠落していれば Caller は確定しない。リポジトリ名にリポジトリ名の文字種は求めない。Repository ID は監査と保護対象の認可に、リポジトリ名は保護対象の認可だけに使う。ワークフロー実行 ID と試行番号は監査用の属性であり、認可には使用しない

**根拠**：事実：Requirements FR-AUTHN-001、FR-AUTHZ-002、GATES OIDC JWT Claims の GOJ-SUB-FORMAT。名前を Repository ID と組になったものに限るのは推論：同じ属性の中で ID と組になっていれば、別のリポジトリの名前を取り違えないため。文字種を求めないのは推論：照合する相手の許可 Caller のリポジトリ名が [INV-016 保護対象の一覧](#inv-016-保護対象の一覧)で文字種を満たすため、文字種の外の名前は許可 Caller に当たらず拒否に倒れる。文字種を認証で求めると、保護対象と関係のない要求まで認証失敗にし得る

#### INV-010 Installation

**時点**：取得した応答から作った時点

**常に成り立つこと**：Installation ID とインストール先 Owner ID が [RULE-002 ID の形式](#rule-002-id-の形式)を満たす。停止状態が、停止しているか停止していないかのどちらかに確定しており、停止状態を読み取れない応答からは作らない。インストール先 Owner ID が許可オーナーの Owner ID と一致することと停止していないことは含まず、発行許可の確定が [RULE-011 Installation の有効性](#rule-011-installation-の有効性)として判定する

**根拠**：推論：読み取れない停止状態を停止していないと読むと、停止中の Installation で発行し得る（Fail Closed）。一致しない、または停止している Installation も GitHub 上に実在する値であり、作れない値ではなく発行に使えない値であるため

#### INV-011 発行許可

**時点**：確定した時点

**常に成り立つこと**：[RULE-010 認可条件](#rule-010-認可条件)と [RULE-015 保護対象の認可条件](#rule-015-保護対象の認可条件)をすべて満たす。対象は発行要求の対象と同じ列で、各完全名の表記と並びも発行要求のままである（GitHub API に渡すリポジトリ名と監査記録の要求対象を、利用者の要求と一致させるため）。Permissions は発行要求の Permissions に等しい（上限の水準に上げない）。その Permission Entry はどれも許可権限に含まれ、発行上限に水準の包含で含まれ、保護対象に当たる対象があれば、その許可 Caller の権限の上限にも水準の包含で含まれる。Installation ID は、インストール先 Owner ID が許可オーナーの Owner ID と一致し、停止していない Installation のものである

**根拠**：事実：Security Design §3.1、§3.6、Requirements FR-AUTHZ-008、FR-AUTHZ-009

#### INV-012 返却可能な発行結果

**時点**：照合を通った時点

**常に成り立つこと**：トークンが空でなく、有効期限が将来である。実効対象の完全名の集合が発行許可の対象と一致し、件数が同じで、余分・欠落・重複がない。各実効対象の Repository ID が [RULE-002 ID の形式](#rule-002-id-の形式)を満たして互いに異なり、所有オーナーの Owner ID が許可オーナーの Owner ID と一致する。実効権限が、発行許可の Permissions に付随権限を加えた範囲内で、発行許可の Permissions をすべて含む。範囲と包含は権限名と水準の組で判定し、水準が発行許可と異なる Permission Entry は、高くても低くても条件を満たさない。対象の選び方が、選択したリポジトリに限られている。各実効対象と各保護対象について、実効対象の完全名のリポジトリ名が保護対象のリポジトリ名と一致することと、実効対象の Repository ID が保護対象の Repository ID と一致することが、両方成り立つか、両方成り立たない

**根拠**：事実：Security Design §3.4、Requirements FR-ISSUANCE-004、FR-AUTHZ-010。リポジトリ名の比較は [RULE-005 名前の比較規則](#rule-005-名前の比較規則)。実効権限の照合に水準の包含を使わないのは事実：Requirements FR-ISSUANCE-004（要求した水準との一致を求める）

対象の全件一致は、GitHub が要求外の対象を黙って除いた場合も、余分な対象を加えた場合も検出します。
この条件があるため、FR-AUTHZ-005 のうち「トークンを返さない」ことは GitHub の拒否の振る舞いに依存しません（推論）。
GitHub が発行した後に不整合を検出して失効に失敗した場合、GitHub 側でトークンが有効なまま残り得ます。

#### INV-013 Output

**時点**：利用者接点が成功応答から作った時点

**常に成り立つこと**：`token` が空でなく ASCII の制御文字を含まない。`expires-at` が空でなく改行を含まない。`repositories` と `permissions` が 1 件以上で、各要素が空でなく改行と `,` を含まず、権限名と水準は `:` も含まない。成功応答の `request_id` も空でなく改行を含まない。`token` 以外の値と `request_id` は、旧形式のワークフローコマンドの接頭辞 `##[` を含まない

**根拠**：事実：GitHub Actions Design §2.1、§3。要素の文字の条件は推論：カンマ区切りの Output を読む後続ステップが、`,` や `:` を含む要素を別の要素として読むため

#### INV-014 発行処理

**時点**：全体

**常に成り立つこと**：返却可能な発行結果がなければトークンを返さない。照合を通らず、空でないトークン文字列を取得できた場合は、実行可能な範囲で失効 API の呼び出しを試みる。失効成功、応答を取得できなかったトークンの不存在、中断した場合の失効の試行は保証しない。固定設定は処理中に変わらない。資格情報をログとエラー本文に出さない

**根拠**：事実：Requirements QR-003、FR-AUTHZ-005、FR-ISSUANCE-004、FR-OPS-001、QR-004

## Bounded Contexts and Ubiquitous Language

### Contexts

同じ語が境界をまたぐと意味を変えるため、次の 5 つの範囲を別の Bounded Context として扱います。

| Bounded Context | 英語名 | 担うこと | 実装 | 中心の語 |
|---|---|---|---|---|
| **認証** | Authentication | 未検証の GATES 用 OIDC JWT を、Issuer の公開鍵と固定設定の Issuer と Audience で検証し、Caller を確定する | Lambda の Go アプリケーションのうち OIDC Verifier | JWT、JOSE Header、Claim、主体（`sub`）、Issuer、Audience、Discovery、JWK / JWKS、`kid`、使用可能な鍵 |
| **発行** | Issuance | 発行要求と Caller から発行許可を確定し、発行結果を照合して返す | Lambda の Go アプリケーションのうち OIDC Verifier を除く部分 | 本ドキュメントの Concepts の概念のうち、Issuer、GATES 用 Audience、Issuer の公開鍵集合、Discovery を除くもの |
| **入口制御** | Entry Control | 許可オーナーのワークフローだけに Lambda の Invoke を許す | IAM OIDC Provider、IAM Role、Terraform | OIDC JWT の主体の文字列パターン、Audience、AWS 一時認証情報、`lambda:InvokeFunction` |
| **利用者接点** | Workflow Integration | 利用者の入力を Invoke Payload に変換し、OIDC JWT の取得と IAM Role の引き受けを経て Lambda を Invoke し、Lambda の応答を判定して発行結果を Output として後続ステップへ渡し、取り出したトークンの失効（失敗時の失効、ジョブの終了時の失効）を試みる | Docker Action | `repositories` と `permissions` のカンマ区切り、接続情報、要求用トークン、OIDC JWT、AWS 一時認証情報、Invoke の結果、成功応答と失敗応答、Output、通信の失敗、キャンセルの要求、ワークフローコマンド（マスク、注釈）、ファイルコマンド（`GITHUB_OUTPUT`、`GITHUB_STATE`） |
| **鍵管理** | Key Custody | KMS Key の作成、鍵素材のインポート、鍵の更新 | Terraform と独立した運用手順 | KMS Key、鍵素材、`PendingImport` と `Enabled`、`key_spec`、鍵の集合、使用中の鍵 |

GitHub は、オーナー、リポジトリ、GitHub App、Installation を所有する上流です。
発行 Context は GitHub の API の語に従いつつ、完全名の列と GitHub API に渡すリポジトリ名の配列の間を翻訳します（事実：Lambda Design §6.3）。

認証 Context は、GitHub OIDC Provider の語（Claim と主体の文字列）を発行 Context の語（Owner ID、Repository ID、リポジトリ名、ワークフロー実行 ID、試行番号、ワークフロー起動属性）に翻訳します。
Caller のリポジトリ名は、主体の文字列のリポジトリ部分の名前です。
認証 Context はこの名前をどの名前とも照合せず、発行 Context が保護対象の認可で照合します（事実：GATES OIDC JWT Claims の GOJ-SUB-FORMAT）。
認証から発行へ渡る値は Caller と署名検証済みの Claims Set だけです。
発行 Context は GATES 用 OIDC JWT を読まず、署名検証済みの Claims Set は解釈せずに監査記録へ書き写します。
固定設定のうち Issuer と Audience は認証 Context が、それ以外は発行 Context が読みます。
失敗の種別は両 Context で共通です。

利用者接点は、利用者の入力を Invoke Payload に変換するために、リストの構造（空の要素、区切りの数、重複）と、要素が UTF-8 として正しいことだけを確かめます（事実：GitHub Actions Design §2.2）。
対象の重複は [RULE-005 名前の比較規則](#rule-005-名前の比較規則)で、権限名の重複は文字列の完全一致で判定します。
変換では要素の前後の空白を除くだけで、大文字小文字をそろえず、入力の順のまま Invoke Payload に入れます（事実：GitHub Actions Design §2.2）。
値を書き換えないのは、利用者が書いた要求と監査記録に残る要求を一致させるためです（推論）。
完全名と権限の文字種、対象の上限、認可条件は発行 Context の規則であり、利用者接点は持ちません。
利用者接点が同じ規則を持つと定義が 2 か所になり、発行 Context の規則だけを変えたときに、発行 Context が受け付ける要求を利用者接点が拒否し得るためです（推論）。

利用者接点は、Lambda の Invoke API が返した **Invoke の結果**（Invoke Result。`StatusCode`、`FunctionError`、Payload）を、AWS SDK のエラーとあわせて判定（Judge）します（事実：GitHub Actions Design §3）。
Payload を JSON として読み、`ok` が `true` なら **成功応答**（Success Response）、`false` なら **失敗応答**（Failure Response）です（事実：Lambda Design §3）。
成功応答からは `token` を Installation Access Token として取り出し（取り出すのは、空でなく ASCII の制御文字を含まない文字列だけ）、残りの項目をすべて読み取れたときだけ **Output**（Output。`token`、`expires-at`、`repositories`、`permissions`）を作ります（事実：GitHub Actions Design §2.1、§3）。
失敗応答のエラーコードは発行 Context の失敗の種別を示すもので、利用者接点はそれを読み替えずに示します（事実：GitHub Actions Design §3.2）。
OIDC JWT の取得、AWS STS、Invoke、失効のどの呼び出しでも、応答を得られなかった失敗を **通信の失敗**（Communication Failure）と呼び、タイムアウトとそれ以外に分けて示します（事実：GitHub Actions Design §3.2）。
発行 Context では通信の失敗は一時的な障害に含まれますが、利用者接点は失敗の種別を持たず、通信の失敗をそのまま示します（推論：利用者接点は再試行せず、どの失敗もステップの失敗として利用者に伝え、再実行するかを利用者に委ねるため）。

利用者接点は、発行要求を自動で送り直しません（事実：GitHub Actions Design §3.3）。
応答を受け取れなかった発行処理でも、GitHub 側でトークンが発行されて残り得るため（[Lifecycle of an Issuance](#lifecycle-of-an-issuance)）、送り直すと誰も持たないトークンが増え得るからです（推論）。

利用者接点は、ジョブの終了時の処理へ引き継いだ Installation Access Token の失効を、ジョブの終了時にそのトークン自身で認証して試みます（事実：Requirements FR-ISSUANCE-006、GitHub Actions Design §3.5）。
利用者接点はトークンを後続ステップへ渡す前にジョブの終了時の処理へ引き継ぐため、後続ステップへ渡したトークンはすべてこの対象に含まれます（事実：GitHub Actions Design §2.1）。
この操作を **ジョブの終了時の失効** と呼び、発行 Context の失効（照合を通らなかった発行結果のトークンに対し、発行処理の不変条件が試行を求めるもの）と区別します。
GitHub API の操作は同じで、英語名はどちらも Revoke です。
ジョブの終了時の失効はベストエフォートで、成否はワークフローのログにだけ残り、監査記録の失効の結果には含まれません（推論：監査記録の単位は発行処理 1 回であり、発行処理は返却可能な発行結果を返した時点で終わっているため）。
失効の要求の失敗に限らず、失効を試みる処理がどのように失敗してもジョブの結果を変えず、トークンは GitHub 側で有効なまま、GitHub が定めた有効期限まで残り得ます（事実：GitHub Actions Design §3.2、§3.5）。
ジョブの終了時の処理へトークンを引き継がなかったジョブでは何もしません。

利用者接点がトークンを取り出した後に失敗で終える場合は、キャンセルで終える場合も含め、そのトークンの失効をその場で試みます（事実：Requirements FR-ISSUANCE-006、GitHub Actions Design §3.5）。
この操作を **失敗時の失効** と呼びます。
ジョブの終了時の処理へ引き継ぐ前に終わると、ジョブの終了時の失効の対象にもならず、誰も持たないトークンが有効期限まで残るためです（推論）。
取り出せなかったトークンは利用者接点が持たないため、どちらの失効の対象にもなりません。
引き継いだ後に失敗した場合は、同じトークンが失敗時の失効とジョブの終了時の失効の両方の対象になります（事実：GitHub Actions Design §3.5）。
失敗時の失効もベストエフォートで、成否はワークフローのログにだけ残り、成否にかかわらず利用者接点は失敗で終わります。

どちらの失効についても、利用者が止める手段は設けません。利用者接点はジョブをまたぐトークンの受け渡しを提供せず、ジョブの終了後にトークンを使う正当な経路がないためです（事実：GitHub Actions Design §2.1）。

```mermaid
flowchart LR
  GH[GitHub<br/>オーナー、リポジトリ、App、Installation]
  UI[利用者接点<br/>Docker Action]
  EC[入口制御<br/>IAM / STS]
  AN[認証<br/>OIDC Verifier]
  IS[発行<br/>Lambda]
  KC[鍵管理<br/>KMS / Terraform]
  UI -->|Invoke Payload の契約| IS
  EC -->|Invoke を許す主体を限定| IS
  IS -->|未検証の GATES 用 OIDC JWT| AN
  AN -->|Caller| IS
  KC -->|KMS Key ARN と Sign だけを公開| IS
  GH -->|OIDC JWT| UI
  GH -->|OIDC JWT| EC
  UI -->|AWS 用 OIDC JWT| EC
  EC -->|AWS 一時認証情報| UI
  GH -->|Discovery と公開鍵集合| AN
  GH -->|Installation と発行結果| IS
  IS -->|発行結果| UI
  UI -->|失敗時の失効、ジョブの終了時の失効| GH
```

入口制御と認証は、同じ GitHub OIDC Provider を信頼しますが、互いの判定を前提にしません。
AWS 一時認証情報と GATES 用 OIDC JWT が同じジョブで取得されたことは検証されず、発行の可否は GATES 用 OIDC JWT と固定設定から判断します（事実：Security Design §2.2）。
この独立があるため、入口制御の資格情報が盗まれても、それだけでは Caller を確定できません。

入口制御が拒否した呼び出しは、発行処理を始めません。
Invoke を許可されない主体からの Invoke、AWS の認証情報のない呼び出し、OIDC JWT の主体が Trust Policy の条件に合わず IAM Role を引き受けられないワークフロー（主体が名前だけの旧形式である場合を含む）がこれに当たります。
Lambda の Invoke API が JSON として読めずに拒否した Payload も、関数に届かないため同じです（事実：Lambda Design §2.2）。
これらは失敗の種別に属さず、監査記録も残りません（推論：失敗の種別と監査記録は、発行処理 1 回の単位で定めるため）。

### Terms Whose Meaning Changes Across Contexts

| 語 | 認証 | 発行 | 入口制御 | 利用者接点 | 鍵管理 |
|---|---|---|---|---|---|
| トークン | GATES 用 OIDC JWT | GitHub App JWT、Installation Access Token。GATES 用 OIDC JWT は認証へ渡すだけで読まない | AWS 用 OIDC JWT、AWS 一時認証情報 | 上記すべてと、要求用トークン、Output の `token`、`GITHUB_STATE` に書く値 | 現れない |
| オーナー | GATES 用 OIDC JWT の主体と Claim に現れる不変 Owner ID | 不変 Owner ID で同定する実体。名前と種別は設定値 | OIDC JWT の主体の先頭に現れる、オーナー名と Owner ID を並べた文字列パターン | 完全名の `owner` 部分 | 現れない |
| 主体 | GATES 用 OIDC JWT の `sub`。オーナー名と Owner ID、リポジトリ名と Repository ID を含む文字列 | 現れない。認証が確定した呼び出し元は Caller と呼ぶ | AWS 用 OIDC JWT の `sub` と、Invoke を許す IAM の主体 | 現れない | 現れない |
| Issuer | GitHub Actions の OIDC Provider。固定設定の値で、Discovery と公開鍵の取得先 | 現れない。コンポーネント名の Token Issuer は Installation Access Token の発行を担うもので、Issuer とは別の概念 | IAM OIDC Provider が信頼する発行元 | 現れない | 現れない |
| 検証と照合 | 署名と Claim の検証（Verify） | 発行結果と発行許可の照合（Verify）。英語名が同じため、日本語では「検証」と「照合」で区別する | 現れない | 入力のリストの構造の検証。署名と Claim の検証も、照合もしない | 現れない |
| 権限 / permissions | 現れない | GitHub App の Permissions（`contents:read` など） | IAM の許可（`lambda:InvokeFunction`、`kms:Sign`） | 入力の `permissions`（カンマ区切り）とワークフローの `permissions: id-token: write` | KMS Key Policy の署名権限と管理権限 |
| 応答 | 現れない | GitHub API の応答（発行応答など）と、Lambda Handler が書く成功応答と失敗応答 | 現れない | Lambda Handler が書いた成功応答と失敗応答を、Invoke の結果の Payload として読むもの | 現れない |
| 発行要求 | 現れない | 形式検証を通った対象と Permissions（[INV-001 発行要求](#inv-001-発行要求)）。GATES 用 OIDC JWT を含まない | 現れない | リストの構造と UTF-8 の正しさだけを確かめた、入力の表記と順序のままの対象と権限。文字種と対象の上限は確かめていない | 現れない |
| repositories | 現れない | 発行要求の完全名の列。GitHub API へはリポジトリ名の配列に変換する | 現れない | 入力と Output のカンマ区切り文字列 | 現れない |
| リポジトリ名 | 主体のリポジトリ部分の、Repository ID と組になった名前。どの名前とも照合しない | 完全名のリポジトリ部分、保護対象と許可 Caller のリポジトリ名、Caller のリポジトリ名。保護対象を発行前に見分け、許可 Caller の照合で ID に重ねて使う | 現れない | 完全名の `repo` 部分 | 現れない |
| 鍵 | Issuer の公開鍵（検証だけに使う） | 署名鍵の KMS Key ARN（署名だけを依頼する） | 現れない | 現れない | 鍵素材、KMS Key、Private Key のコピー |
| 失効 | GATES 用 OIDC JWT の失効時刻（`exp`）。時刻の経過で効力を失うこと | 照合を通らなかった発行結果のトークンを GitHub API で無効にする操作（Revoke）。発行処理の不変条件が試行を求め、失効の結果を監査記録に残す | 現れない | 取り出した Installation Access Token を GitHub API で無効にする操作（Revoke）。「ジョブの終了時の失効」と「失敗時の失効」がある。どちらもベストエフォートで、結果はワークフローのログにだけ残る | GitHub 側での旧 Private Key の失効。GATES は代行しない |

## Unverified GitHub Behavior

公式文書で確認できない GitHub の振る舞いのうち、GATES のルールに影響するものを、扱いごとに以下に分けます。
どれも、GitHub は保証していません。
確定できていない事項は、[GitHub Actions OIDC Constraints](../external/github/oidc.md) では Evidence が Unconfirmed の条項に、[GitHub Owner and Repository Constraints](../external/github/account.md)、[GitHub App Constraints](../external/github/app.md)、[GitHub Actions Integration Constraints](../external/github/actions.md) では Outstanding Verification にあります。

範囲外の対象や権限にトークンを返さないことを変えないため、裏取りを前提にしない振る舞いは次のとおりです。

- 範囲外の対象を含む発行 API の呼び出しへの応答。失敗の種別の分類だけが変わる（[RULE-011 Installation の有効性](#rule-011-installation-の有効性)）
- 要求にかかわらず `metadata:read` が付与されるかどうか。付与されなくても照合は通る（[Request and Decision](#request-and-decision)）
- Installation に付与されていない権限か水準を含む発行 API の呼び出しへの応答。拒否でも、権限を除くか水準を下げた発行でも、トークンを返さない。失敗の種別の分類だけが変わる（[INV-012 返却可能な発行結果](#inv-012-返却可能な発行結果)）

正当な要求が拒否されるかを変えるため、実環境で確かめた振る舞いは次のとおりです。

- 要求した権限に応じて、`metadata:read` のほかの権限が発行結果に加わるかどうか。加わると、付随権限に含まれないため照合が失敗する（[INV-012 返却可能な発行結果](#inv-012-返却可能な発行結果)）
- GATES 用 OIDC JWT が示すリポジトリ名が、GitHub がその JWT を発行した時点の GitHub 上の名前と、大文字小文字の違いを除いて同じであるかどうか。異なると、許可 Caller のリポジトリのワークフローの要求でも、Caller が許可 Caller に当たらず認可拒否になる。名前の照合は Repository ID の一致に重ねる条件のため、許可を広げる側には働かない（[RULE-015 保護対象の認可条件](#rule-015-保護対象の認可条件)）
- GATES 用 OIDC JWT の Repository ID が、REST API で運用者が調べる Repository ID と同じ値であるかどうか。異なると、許可 Caller に書いた ID がどの Caller とも一致せず認可拒否になる（[RULE-015 保護対象の認可条件](#rule-015-保護対象の認可条件)）

保護対象と許可 Caller の同定の前提で、実環境では確かめられない振る舞いは次のとおりです。

- Repository ID が、ほかのリポジトリに再び割り当てられないかどうか。再び割り当てられ、そのリポジトリが許可 Caller と同じ名前を持つと、許可 Caller として扱われる。名前が異なれば、保護対象でも許可 Caller でも拒否に倒れる（[INV-012 返却可能な発行結果](#inv-012-返却可能な発行結果)、[RULE-015 保護対象の認可条件](#rule-015-保護対象の認可条件)）

## References

- [Constitution](../CONSTITUTION.md)
- [Requirements](../spec/requirements.md)
- [Acceptance Criteria](../spec/acceptance-criteria.md)
- [Security Design](security.md)
- [Lambda Design](lambda.md)
- [System Architecture](architecture.md)
- [GitHub Actions Design](github-actions.md)
- [AWS Infrastructure Design](aws-infrastructure.md)
- [GitHub Actions OIDC Constraints](../external/github/oidc.md)
- [GitHub Owner and Repository Constraints](../external/github/account.md)
- [GitHub App Constraints](../external/github/app.md)
- [GitHub Actions Integration Constraints](../external/github/actions.md)
- [External](../external/README.md)
- [GATES OIDC JWT Claims](gates-oidc-jwt-claims.md)
- [GATES OIDC JWT Signature](gates-oidc-jwt-signature.md)
- [GitHub — REST API endpoints for GitHub Apps](https://docs.github.com/en/rest/apps/apps)
- [OpenID Connect Discovery 1.0](https://openid.net/specs/openid-connect-discovery-1_0.html)
- [RFC 7517 — JSON Web Key (JWK)](https://www.rfc-editor.org/rfc/rfc7517)

## Appendix: Impact of Future Scope

[Scope](../spec/scope.md) が将来取り込むことを想定する機能を導入する場合、本ドキュメントには次の影響が及びます。

- **細粒度の認可の残る部分**：アクセスされる側のリポジトリの管理者が制御するなら、保護対象の一覧とは決める主体が異なる条件が加わり、RULE-015 と保護対象の一覧の定義が変わる。Caller をリポジトリより細かく見分けるなら、ワークフロー起動属性を監査のためだけに添えるという Caller の定義が変わる。保護対象の一覧にないリポジトリを既定で拒否するなら、RULE-015 の例外がなくなる
- **Organization permissions、Account permissions、Enterprise permissions の発行**：許可権限と付随権限、返却可能な発行結果（INV-012）が変わる。リポジトリを対象とする発行要求の概念も見直す
