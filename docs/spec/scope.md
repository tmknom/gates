# Scope

## 1. Purpose

本ドキュメントは、GATES の現在のスコープ、将来取り込むことを想定している機能、対象外とする範囲を示します。
GATES が満たす要求は [Requirements](requirements.md) に定めるため、本ドキュメントを更新しただけでは GATES への要求は変わりません。

## 2. Current Scope

許可された単一のオーナー配下のワークフローが、Installation に含まれるリポジトリを、GitHub App に付与された Repository permissions の範囲で操作するためのトークンを取得できるところまでを実現します。
次の表の項目は、恒久的な原則ではなく、現在の選択です。

| 項目 | 現在のスコープ | 理由 |
|---|---|---|
| **認可の粒度** | オーナー単位で認可する。Caller やチーム、対象リポジトリごとに、アクセス先と要求できる権限を制限しない | 認可ポリシーの管理と運用を増やさないため |
| **発行可能な権限** | Repository permissions を許可する（[Requirements](requirements.md) の FR-AUTHZ-003）。Organization permissions、Account permissions、Enterprise permissions は発行しない | リポジトリを横断する自動化には、読み取り以外の権限も要るため。リポジトリを単位としない権限は、対象リポジトリの明示では影響範囲を限定できないため発行しない |

この組み合わせでは、許可オーナーのどのワークフローも、Installation に含まれるどのリポジトリに対しても、GitHub App に付与された Repository permissions のすべてを要求できます。
アクセスされる側のリポジトリは、自身に対する操作を GATES の認可では制限できません。
GATES が発行する権限の上限を決めるのは、GitHub App に付与する権限と Installation の対象だけであり、どちらも GitHub 側の設定です（[Requirements](requirements.md) §2 の GitHub App / オーナーの管理者の責任）。
オーナー内の横断的な操作は、現在のスコープに伴う残留リスクとして受容します。
恒久的に受け入れたものではなく、細粒度の認可（[§3](#3-deferred-capabilities)）を取り込むときに見直します。

## 3. Deferred Capabilities

次の機能は現在のスコープに含めず、将来取り込むことを想定します。
優先順位と実施時期は未定です。

| 項目 | 想定する拡張 |
|---|---|
| **細粒度の認可** | Caller やチーム、アクセス先、権限に応じて認可を制御する。アクセスされる側のリポジトリが、自身に対する操作を制御できるようにすることを含む |
| **Organization permissions、Account permissions、Enterprise permissions の発行** | リポジトリを単位としない権限を発行できるようにする |
| **OIDC JWT の再利用の検出** | 有効期限内に同じ OIDC JWT を使った要求を検出する。現在は、再利用できる期間を [Requirements](requirements.md) の FR-AUTHN-002 の時間条件だけで制限している |
| **レート制限と濫用の検出** | 連続した要求を制限し、濫用を検出する |
| **カスタマイズした `sub` の受理** | `include_claim_keys` でカスタマイズした `sub` の GATES 用 OIDC JWT を受理する。現在は、`repo` の部分で始まり、その後に `:` が続く形式だけを受理する。IAM Role の引き受けも `sub` の先頭が許可オーナーの不変 Owner ID を示すことを求めるため（[Requirements](requirements.md) の FR-AUTHN-005）、`repo` の部分で始まらない形式に広げるときは、引き受けの条件もあわせて検討する |

Organization permissions、Account permissions、Enterprise permissions は、Repository permissions と影響範囲が異なり、対象リポジトリを明示しても範囲が狭まらないため、Repository permissions と同じ認可で扱えるとは仮定しません。
これらを発行するときは、現在のオーナー単位の認可のまま広げず、認可の粒度と一体として検討します。

細粒度の認可は、互いに信頼できないチームが同じ許可オーナーを共有するときや、一部のワークフローにだけ操作させたいリポジトリを Installation に含めるときに検討を始めることを想定しています。

## 4. Out of Scope

次の範囲は GATES の対象外とし、将来の導入も想定しません。

- 複数のオーナーの同時サポート
- 複数の GitHub App Installation の同時サポート
- GitHub Enterprise Server など GitHub.com 以外の GitHub への対応
- IAM 層で拒否された呼び出し（[Requirements](requirements.md) の FR-OPS-007）の監査記録。記録されるかどうかは AWS の仕様に依存し、GATES は保証しない
