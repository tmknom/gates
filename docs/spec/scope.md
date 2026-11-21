# Scope

## 1. Purpose

本ドキュメントは、GATES の現在のスコープ、将来取り込むことを想定している機能、対象外とする範囲を示します。
GATES が満たす要求は [Requirements](requirements.md) に定めるため、本ドキュメントを更新しただけでは GATES への要求は変わりません。

## 2. Current Scope

許可された単一のオーナー配下のワークフローが、Installation に含まれるリポジトリを、GitHub App に付与され、発行上限に含まれる Repository permissions の範囲で操作するためのトークンを取得できるところまでを実現します。
**発行上限** は、GATES 運用者がデプロイ時の設定で定める、そのデプロイで発行してよい権限の範囲です（[Requirements](requirements.md) の FR-AUTHZ-008）。
**保護対象の一覧** は、GATES 運用者がデプロイ時の設定で定める、要求できる Caller と権限を限るリポジトリの集合です（[Requirements](requirements.md) の FR-AUTHZ-009）。
次の表の項目は、恒久的な原則ではなく、現在の選択です。

| 項目 | 現在のスコープ | 理由 |
|---|---|---|
| **認可の粒度** | オーナー単位で認可し、GATES 運用者が保護対象の一覧に指定したリポジトリに限って、要求できる Caller と権限を制限する（[Requirements](requirements.md) の FR-AUTHZ-009）。Caller はリポジトリ単位で見分ける。一覧にないリポジトリは、Caller ごとに制限しない。発行上限はデプロイに一つで、どの Caller にも対象リポジトリにも同じに効く | 操作できるワークフローを一部のリポジトリのものに限りたいリポジトリを、Installation に含めたまま守るため。認可ポリシーを、守りたいリポジトリの分だけに限り、中央の一つの設定で管理するため |
| **発行可能な権限** | Repository permissions を許可する（[Requirements](requirements.md) の FR-AUTHZ-006）。そのうち発行するのは、発行上限に含まれるものに限る。Organization permissions、Account permissions、Enterprise permissions は発行しない | リポジトリを横断する自動化には、読み取り以外の権限も要るため。リポジトリを単位としない権限は、対象リポジトリの明示では影響範囲を限定できないため発行しない |

この組み合わせでは、許可オーナーのどのワークフローも、Installation に含まれ、保護対象の一覧にないどのリポジトリ（新しく作ったリポジトリを含む）に対しても、GitHub App に付与され、発行上限に含まれる Repository permissions のすべてを要求できます。
保護対象の一覧にないリポジトリは、そのリポジトリに対する操作を GATES の認可では制限できません。
GATES が発行するトークンの対象と権限の範囲を決めるのは、GitHub App に付与する権限、Installation の対象、発行上限、保護対象の一覧です。
前の 2 つは GitHub 側の設定（GitHub App / オーナーの管理者の責任）、後の 2 つは GATES のデプロイ時の設定（GATES 運用者の責任）です。
発行上限は、どのワークフローに対しても同じ範囲を狭めるだけで、オーナー内の横断的な操作そのものは防ぎません。
保護対象の一覧は、指定したリポジトリへの要求を、**許可 Caller**（保護対象ごとに指定する、要求を許す Caller のリポジトリと権限の上限の組）に当たる Caller からの、その権限の上限の範囲の要求に限ります。
許可 Caller のリポジトリのワークフローを書き換えられる者は、その許可 Caller として、権限の上限の範囲で保護対象を要求できます。
この経路と、保護対象の一覧にないリポジトリへのオーナー内の横断的な操作は、現在のスコープに伴う残留リスクとして受容します。
恒久的に受け入れたものではなく、細粒度の認可の残る部分を取り込むときに見直します。

## 3. Deferred Capabilities

次の機能は現在のスコープに含めず、将来取り込むことを想定します。
優先順位と実施時期は未定です。

| 項目 | 想定する拡張 |
|---|---|
| **細粒度の認可** | 保護対象の一覧で GATES 運用者が中央で制限する範囲を超えて、認可を制御する。アクセスされる側のリポジトリの管理者が、そのリポジトリに対する操作を制御できるようにすること、チームに応じて制御すること、Caller をリポジトリより細かく（ブランチ、イベント、Environment、ワークフロー）見分けること、保護対象の一覧にないリポジトリを既定で拒否することを含む |
| **Organization permissions、Account permissions、Enterprise permissions の発行** | リポジトリを単位としない権限を発行できるようにする |
| **OIDC JWT の再利用の検出** | 有効期限内に同じ OIDC JWT を使った要求を検出する。現在は、再利用できる期間を [Requirements](requirements.md) の FR-AUTHN-001 の時間条件だけで制限している |
| **レート制限と濫用の検出** | 連続した要求を制限し、濫用を検出する |
| **カスタマイズした `sub` の受理** | `include_claim_keys` でカスタマイズした `sub` の GATES 用 OIDC JWT を受理する。現在は、`repo` の部分で始まり、その後に `:` が続く形式だけを受理する。IAM Role の引き受けも `sub` の先頭が許可オーナーの不変 Owner ID を示すことを求めるため（[Requirements](requirements.md) の FR-AUTHN-003）、`repo` の部分で始まらない形式に広げるときは、引き受けの条件もあわせて検討する |

Organization permissions、Account permissions、Enterprise permissions は、Repository permissions と影響範囲が異なり、対象リポジトリを明示しても範囲が狭まらないため、Repository permissions と同じ認可で扱えるとは仮定しません。
これらを発行するときは、現在のオーナー単位の認可のまま広げず、認可の粒度と一体として検討します。

細粒度の認可の残る部分は、互いに信頼できないチームが同じ許可オーナーを共有するときや、守りたいリポジトリが増えて、保護対象の一覧を中央で保つ運用が追いつかなくなったときに検討を始めることを想定しています。

## 4. Out of Scope

次の範囲は GATES の対象外とし、将来の導入も想定しません。

- 複数のオーナーの同時サポート
- 複数の GitHub App Installation の同時サポート
- GitHub Enterprise Server など GitHub.com 以外の GitHub への対応
- IAM 層で拒否された呼び出し（[Requirements](requirements.md) の FR-OPS-002）の監査記録。記録されるかどうかは AWS の仕様に依存し、GATES は保証しない
