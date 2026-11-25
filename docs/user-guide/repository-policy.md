# Repository Policy

GATES のトークンで操作されるリポジトリの管理者に向けて、どの Caller にどの権限を許すかを決めるリポジトリポリシーの書き方と、許す前に知っておくことを示します。
リポジトリポリシーが効くのは、運用者がリポジトリポリシーを適用する設定でデプロイした GATES だけです。
適用しているかは、運用者に確かめます。

## 1. What the Repository Policy Decides

リポジトリポリシーを適用するデプロイでは、GATES はリポジトリへの要求のたびに、そのリポジトリのリポジトリポリシーを読みます。
リポジトリポリシーが許した Caller から、許した権限の範囲で要求されたときだけ、そのリポジトリを含むトークンを発行します。

- **ファイルがなければ、どの Caller にも発行しない**：リポジトリポリシーを置いていないリポジトリを含む要求は、どのワークフローからでも拒否される。リポジトリ自身のワークフローが自分のリポジトリを要求する場合も同じである
- **ほかの設定を越えない**：リポジトリポリシーで許しても、GitHub App の権限、発行上限、インストール対象、運用者の保護対象の一覧が許さない要求は通らない
- **一つでも許されなければ全体を拒否する**：複数のリポジトリを一度に要求したとき、どれか一つのリポジトリポリシーが許さなければ、要求全体が失敗する
- **変えても、発行済みのトークンは失効しない**：Caller を外しても、権限を狭めても、それまでに発行されたトークンは、Action がジョブの終わりに試みる失効が成功するか、期限（発行から 1 時間）が来るまで、発行したときの権限で使える

## 2. Writing the File

既定ブランチの `.github/gates-policy.yml` に置きます。
既定ブランチに入った後の要求から効きます。
ほかのブランチのファイルは読まれません。

次の例は、`deploy-tools` のリポジトリのワークフローに、`contents:read` と `pull_requests:write` までを許します。

```yaml
callers:
  # 共通のビルド基盤
  - repository_id: "100"
    repository_name: deploy-tools
    permissions:
      contents: read
      pull_requests: write
```

どの Caller にも許可しないときは、許可 Caller を空にします。
空のファイルは不正として扱われ、結果は同じく拒否ですが、誤りと見分けられるように `callers: []` と書きます。

```yaml
callers: []
```

| キー | 書くもの |
|---|---|
| `callers` | 許可 Caller の列。`[]` は、どの Caller にも許可しないことを示す |
| `repository_id`、`repository_name` | Caller のリポジトリの Repository ID と、オーナー名を含まないリポジトリ名 |
| `permissions` | その Caller に許す権限の上限。権限名をキー、許す最も高い水準を値にして、1 組以上書く |

Caller のリポジトリの Repository ID と名前は、次のコマンドがこの順に出力します。
画面の表示や記憶から書かず、出力を写します。

```bash
gh api repos/<オーナー名>/<リポジトリ名> --jq '[.id, .name] | @tsv'
```

```text
1296269	Hello-World
```

GATES は Repository ID と名前の両方が一致する Caller だけを許可 Caller とみなします。
名前を求めるのは、変更をレビューする人が名前で判断するためです。
名前を書かずに ID だけで許すと、名前の示すリポジトリと違うリポジトリを許す書き誤りに気付けません。

書くときは、次を守ります。

- **権限名と水準は [Action Reference](action-reference.md) の「要求できる権限」にあるものにする**：GitHub App の設定画面の表示名（`Pull requests` など）ではなく `pull_requests` のように書く
- **権限名ごとに、必要な最も低い水準を書く**：`write` を書くと、同じ権限名の `read` の要求も許す
- **ID は引用符で囲む**：`repository_id: "100"` のように書く。引用符のない ID は不正になる
- **文字列以外に読める名前は引用符で囲む**：`123` や `true` のような名前を引用符なしで書くと不正になる。迷ったら囲む
- **アンカーなどの YAML の機能を使わない**：アンカー（`&`）、エイリアス（`*`）、マージキー（`<<`）、タグ（`!!str` など）、同じキーの重複、複数の文書（`---` で区切ったもの）は不正になる
- **同じ Caller を 2 回書かない**：Repository ID かリポジトリ名が重複すると不正になる。リポジトリ名は大文字小文字を区別せずに比べる
- **64 KiB に収める**

誤りが一部でもあると、正しく書いた許可 Caller を含めて全体が無効になり、どの Caller の要求も拒否されます。

## 3. Before Allowing a Caller

### 3.1 既定ブランチに書き込める人は、許可を変えられる

リポジトリポリシーは、既定ブランチに書き込めるだれでも書き換えられます。
リポジトリの Admin である必要はなく、GATES はだれが変更したかを確かめません。
変更できる人を絞りたいときは、GitHub の次の機能を使います。

- **ブランチ保護か Rulesets**：既定ブランチへの直接の push を禁じ、Pull Request のレビューを求める
- **CODEOWNERS**：`.github/gates-policy.yml` のコードオーナーを決め、ブランチ保護か Rulesets でコードオーナーのレビューを必須にすると、このファイルを変える Pull Request は、コードオーナーが承認するまで既定ブランチにマージできない

ユーザーアカウントの非公開リポジトリでブランチ保護と CODEOWNERS を使うには、GitHub Pro が要ります。
Organization のリポジトリでも、プランによって使えない機能があります。
使えないプランでは、書き込みの権限を持つ人を絞ることが、リポジトリポリシーを守る手段になります。

`.github/gates-policy.yml` にシンボリックリンクを置くと、リンク先がファイルなら、その内容がリポジトリポリシーとして読まれます。
リンク先を書き換えられる人もリポジトリポリシーを変えられるため、シンボリックリンクにしません。

### 3.2 `contents:write` を許すことは、すべてを許すことと同じ

**ある Caller に `contents:write` を許すと、その Caller はトークンでリポジトリポリシーを書き換え、自分にほかの権限を許せます。**
そのため、`contents:write` を許すことは、GATES が発行できる範囲で、このリポジトリへのすべての権限を許すことに等しくなります。
`administration:write` も、コラボレーターの追加やブランチ保護の解除で既定ブランチに書き込めるようになるため、同じです。

ブランチ保護でも、この書き換えを止められるとは限りません。
この書き換えでも越えられない上限が要るリポジトリは、運用者に保護対象の一覧に加えてもらいます。
保護対象の一覧は運用者が GATES の設定として持ち、リポジトリポリシーを書き換えても変わりません。

### 3.3 Caller はリポジトリ単位で見分ける

GATES は Caller をリポジトリ単位で見分け、ブランチ、イベント、ワークフローは見分けません。
許可 Caller のリポジトリでワークフローを動かせる人は、ブランチや Pull Request から動かすワークフローを含めて、その許可 Caller に許した範囲を使えます。
許す前に、Caller のリポジトリでだれがワークフローを動かせるかを確かめます。

### 3.4 ファイルは読まれ、複製され得る

リポジトリポリシーは、このリポジトリを読めるだれでも読めます。
秘密の情報ではありませんが、どのリポジトリを経由すればこのリポジトリに届くかの手がかりになります。

テンプレートからの生成などで、このリポジトリの内容を複製して新しいリポジトリを作ると、リポジトリポリシーも複製され、新しいリポジトリも同じ Caller と権限を許します。
複製したリポジトリの管理者は、リポジトリポリシーを残すかを確かめます。

## 4. When the Caller Repository Is Renamed or Re-created

Caller のリポジトリを改名すると、`repository_name` を新しい名前に直すまで、そのリポジトリからの要求は拒否されます。
`repository_id` は変わらないため、直しません。

Caller のリポジトリを削除して同じ名前で作り直すと、新しいリポジトリは別の Repository ID を持ち、許可を引き継ぎません。
許すなら、§2 のコマンドで新しい Repository ID を調べて `repository_id` を直します。

同じ Caller を許しているリポジトリが複数あれば、それぞれのリポジトリポリシーを直します。
Caller のリポジトリの管理者が改名の時期を前もって伝えると、許しているリポジトリの管理者が同時に直せるため、拒否される時間を短くできます。

## 5. Checking the File

GATES はファイルの誤りの位置を示さないため、変更の前後に次を確かめます。

1. 既定ブランチに入れる前に、§2 の規則に沿っているかを見直す。とくに、引用符のない ID、表示名で書いた権限名、同じキーの重複、空のファイルに気を付ける
2. 既定ブランチに入れた後、ファイルが既定ブランチにあり、ファイルとして読めることを確かめる。`file` と出力されれば、置き場所は正しい

    ```bash
    gh api repos/<オーナー名>/<リポジトリ名>/contents/.github/gates-policy.yml --jq .type
    ```

3. 許可 Caller のワークフロー利用者に、このリポジトリだけを要求して発行されるかを確かめてもらう

ワークフロー利用者の要求が `error code "authorization_denied"` で失敗しても、注釈からは、どのリポジトリのリポジトリポリシーが何を許さなかったかを見分けられません。
ワークフロー利用者は、要求に含めたリポジトリを 1 件ずつ要求して、拒否されるリポジトリを絞り込めます。
理由まで知りたいときは、注釈の `request_id` を運用者に伝えると、運用者が監査記録から、拒否したリポジトリが要求の何件目かと、拒否の理由の種類（ファイルがない、ファイルでない、大きすぎる、不正、`callers: []` で許可 Caller が空、Caller が許されていない、権限の上限を超える）を調べられます。
Caller が許されていない場合は、Repository ID だけが一致するか、名前だけが一致するかも見分けられ、§4 の改名か作り直しかの手がかりになります。
監査記録にも、リポジトリポリシーの内容は残りません。

## 6. References

- [GitHub — About protected branches](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches)
- [GitHub — About rulesets](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/about-rulesets)
- [GitHub — About code owners](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners)
- [GitHub — GitHub's plans](https://docs.github.com/en/get-started/learning-about-github/githubs-plans)
- [GitHub — Permissions required for GitHub Apps](https://docs.github.com/en/rest/authentication/permissions-required-for-github-apps)
