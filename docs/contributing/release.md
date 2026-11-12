# Release

GATES の Lambda の配布物を GitHub Releases で公開する仕組みの設計と、リリースを行う手順、利用者が配布物を検証する手順です。
ワークフローの処理そのものは [Release のワークフロー](../../.github/workflows/release.yml) と [Reusable Build のワークフロー](../../.github/workflows/reusable-build.yml) を正とし、このドキュメントでは、その処理が守る方針と理由を説明します。

## Trust in Release Assets

利用者が配布物を信頼する根拠は、利用者自身が行う来歴の検証に置きます。
ワークフローの起動の制限やジョブの権限は、誤った操作を防ぎ、被害の範囲を狭めるためのもので、配布物の正しさを保証するものではありません。
利用者は、配布物に付けた GitHub Artifact Attestations の Build Provenance で、配布物がどのコミットからどのワークフローで作られたかを確かめます。
この検証は、GitHub の署名とワークフローの実行環境を信頼して行う確認です。

## Release Assets

| ファイル | 内容 |
| --- | --- |
| `gates-lambda-linux-arm64.zip` | Lambda の関数のコード。linux/arm64 向けの `bootstrap` だけを入れた zip |
| `SHA256SUMS` | zip の SHA-256 |

Attestation の対象は zip だけです。
`SHA256SUMS` は zip と同じ場所から配るため、転送中の破損には気づけますが、配布物の改ざんの検出には使えません。

## Design of the Release Workflows

### Binding the Attestation to the Build

ビルドと Attestation は、同じ Reusable Workflow の中で行います。
利用者は、Attestation の署名者がこの Reusable Workflow であることを確かめ、ビルドした成果物の Attestation であることを確認します。
もし Attestation だけを別のワークフローに分けると、呼び出し側が渡した任意の digest に署名できてしまい、署名者を確かめても配布物がビルドの成果物であることを示せません。

### Least Privilege per Job

各ジョブには、そのジョブの処理に要る権限だけを与えます。
同じ Reusable Workflow の中でも、ビルドのジョブには読み取りの権限だけを与え、Attestation の署名に要る権限は Attestation のジョブにだけ与えます。
ジョブは別々の runner で動くため、ビルドの手順に紛れ込んだ処理があっても、署名の権限を使って任意の digest に署名することはできません。

リリースを作って公開する書き込みの権限は、最後のジョブにだけ与えます。

### Ordering by Reversibility

取り消せない処理ほど後に置きます。

```
test ───┐
        ├─> build ─> attest ─> 配布物の検証 ─> 公開（タグの作成）─> 公開したアセットの検証
version ┘
```

Attestation は公開の透明性ログに残り、取り消せません。
ただし、公開しなかった配布物の Attestation は「この digest はこのコミットからこのワークフローで作った」という正しい記録で、害はありません。
公開の前に、ビルドの成果物を利用者と同じ Attestation の検証にかけるため、利用者が検証できない配布物は公開されません。
公開より前に失敗したときは、リリースもタグも残りません。

タグは、公開のときに GitHub がビルドしたコミットに作ります。
公開の後には、公開したアセットをダウンロードし、同じ検証をもう一度行います。

同じバージョンを 2 つの実行が計算しないよう、ワークフローの実行は重ねません。

### Restricting Who Starts a Release

リリースは、リポジトリのオーナーが main から起動したときだけ動きます。
ほかのブランチやほかの人が起動すると、すべてのジョブが skip されます。
この制限は誤った起動を防ぐためのもので、改ざんへの備えは利用者による Attestation の検証にあります。
書き込みの権限を持つ人は、制限を消したワークフローをほかのブランチから起動できますが、その配布物は main で起動したことを確かめる検証に失敗します。

### Versioning

バージョンは `vX.Y.Z` の形の semver で、前回のタグから計算します。
タグがまだないときは `v0.0.0` から数えます。

前回のタグは、最も新しい semver のタグとみなします。
そのため、リリースのワークフロー以外でタグを作りません。
`v2` のような動くタグや手で作ったタグがあると、前回のバージョンを取り違えます。
また、計算したバージョンのタグがすでにあると、リリースはビルドしたコミットではなく既存のタグに付きます。

### Reproducible Build

同じコミットと同じバージョンの Go からは、同じバイト列の zip ができるようにします。
そのため、ビルドの結果を変えるものを、コミットの内容とワークフローに明示した設定だけに絞り、runner の環境やキャッシュに紛れ込んだものが配布物に入らないようにします。

- Go のバージョンは `go.mod` の `go` の行に合わせ、一致しなければビルドは失敗する
- Go のビルドの設定は、実行する環境から引き継がず、ワークフローに明示した値に固定する
- ビルドしたディレクトリのパスと作業ツリーの状態は、バイナリに入れない。コミットとの対応は Attestation が示す
- zip に記録されるファイルの権限と更新時刻は、コミットから決まる値にし、OS ごとの拡張フィールドは入れない
- 同じリポジトリのほかのワークフローが書き込めるキャッシュは使わない

## Running a Release

リリースは、main で `workflow_dispatch` から起動します。

```bash
gh workflow run release.yml --ref main -f bump-level=auto
```

入力の `bump-level` で、前回のバージョンから上げる位置を選びます。

| 値 | 次のバージョン |
| --- | --- |
| `auto` | 前回のタグからビルドするコミットまでにマージされた PR のラベルで決める。major は選ばず、上げる理由になるラベルがなければ失敗する |
| `patch` | patch を上げる |
| `minor` | minor を上げ、patch を 0 にする |
| `major` | major を上げ、minor と patch を 0 にする |

PR のラベルは、Label のワークフローが Conventional Commits の種類から付けます。

## When a Release Fails

公開より前に失敗したときは、リリースもタグも残らないため、原因を直して起動し直します。

公開の後の検証で失敗したときは、リリースとタグが公開されたまま残ります。
公開したリリースとタグを取り下げてから、原因を調べます。

```bash
gh release delete <バージョン> --yes --cleanup-tag
```

## Verifying Release Assets

配布物の利用者は、チェックサムと Attestation で配布物を確かめます。
Attestation の検証が、配布物の来歴を示す本体です。

### Checksum

配布物をダウンロードし、`SHA256SUMS` と照合します。

```bash
gh release download <バージョン> --repo tmknom/gates
sha256sum -c SHA256SUMS        # macOS では shasum -a 256 -c SHA256SUMS
```

### Attestation

zip の Attestation を `gh attestation verify` で検証します。
`<コミット>` には、リリースのタグが指すコミットの SHA を入れます。
タグはリリースの公開のときに GitHub が作る軽量タグのため、次のコマンドの出力の SHA がそのままコミットの SHA です。

```bash
git ls-remote https://github.com/tmknom/gates.git refs/tags/<バージョン>
```

```bash
gh attestation verify gates-lambda-linux-arm64.zip \
  --repo tmknom/gates \
  --signer-workflow tmknom/gates/.github/workflows/reusable-build.yml \
  --signer-digest <コミット> \
  --source-ref refs/heads/main \
  --source-digest <コミット> \
  --deny-self-hosted-runners
```

| 引数 | 確かめること |
| --- | --- |
| `--repo` | Attestation が GATES のリポジトリで作られたこと |
| `--signer-workflow` | 署名者が、ビルドと Attestation を行う Reusable Workflow であること |
| `--signer-digest` | 署名した Reusable Workflow が、ビルドしたコミットのものであること。Reusable Workflow は同じリポジトリから呼ぶため、`--source-digest` と同じコミットになる |
| `--source-ref` | main で起動したワークフローで作られたこと |
| `--source-digest` | リリースのタグが指すコミットからビルドされたこと |
| `--deny-self-hosted-runners` | GitHub が用意する runner でビルドされたこと |

検証が成功すると、コマンドは終了コード 0 で終わります。
どれかの条件に合わないときは、0 以外の終了コードで終わります。

### Lambda Function Code

リリースの zip をそのまま Lambda に上げたときは、Lambda の関数の `CodeSha256` とも照合できます。
`CodeSha256` は zip の SHA-256 を Base64 で表した値です。

```bash
openssl dgst -sha256 -binary gates-lambda-linux-arm64.zip | openssl base64
```
