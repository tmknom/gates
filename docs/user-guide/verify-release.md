# Release Verification

GATES は、AWS Lambda で動かす関数のコードを zip にして、リリースごとに GitHub Releases に置いています（この zip を、以下、**配布物** と呼びます）。
本ドキュメントでは、**GATES 運用者**（GATES を AWS にデプロイして運用する人。以下、運用者）が、配布物がどこから来たか（**来歴**）を確かめる手順と、デプロイした関数のコードが配布物と同じかを照合する手順を扱います。

配布物を信頼してよいかの根拠は、運用者自身が行う来歴の検証に置きます。
GATES のリリースのワークフローは、起動できる人とジョブの権限を絞っています。
この制限は誤った操作を防いで被害の範囲を狭めるためのもので、配布物の正しさは保証しません。

来歴の検証には、配布物に付いた **Attestation** を使います。
Attestation は、配布物をどのリポジトリのどのコミットから、どのワークフローでビルドしたかを示す、署名付きの記録です（GitHub Artifact Attestations による Build Provenance）。
この検証は、GitHub の署名の仕組みと、ワークフローが動く GitHub の実行環境を信頼することを前提にしています。

## 1. Prerequisites

- GitHub CLI にログインしてある（`gh auth login`）。§3 と §4 の `gh` のコマンドが GitHub の API を呼ぶため
- §5 では、AWS の認証情報に Lambda の関数の情報を読む権限（`lambda:GetFunction`）があり、AWS CLI のリージョン（`AWS_REGION`）を GATES の Lambda の関数と同じリージョンにしてある
- §5 のコマンドは、GATES のリポジトリのルートで実行する

## 2. Release Assets

リリースごとに、次のファイルを置きます。

| ファイル | 内容 |
|---|---|
| `gates-lambda-linux-arm64.zip` | 配布物。Lambda の関数のコードで、linux/arm64 向けの実行ファイル `bootstrap` だけを入れた zip |
| `SHA256SUMS` | 配布物の SHA-256 |

`SHA256SUMS` には Attestation が付いていないため、`SHA256SUMS` との照合で気づけるのは転送中の破損だけです。
`SHA256SUMS` は配布物と同じ場所から配るため、両方を書き換えられると照合は通り、改ざんは検出できません。
改ざんがないことは、§4 の Attestation の検証で確かめます。

## 3. Checking the Checksum

配布物をダウンロードし、`SHA256SUMS` と照合します。
`<バージョン>` には、`vX.Y.Z` の形のリリースのタグを入れます。

```bash
gh release download <バージョン> --repo tmknom/gates
sha256sum -c SHA256SUMS        # macOS では shasum -a 256 -c SHA256SUMS
```

配布物の行に `OK` と出れば一致です。
一致しないときは、ダウンロードからやり直します。
やり直しても一致しない配布物は使いません。

## 4. Verifying the Attestation

配布物の Attestation を `gh attestation verify` で検証します。
`<コミット>` には、リリースのタグが指すコミットの SHA を入れます。
タグはリリースの公開のときに GitHub が作る軽量タグのため、次のコマンドが出力する 1 列目の SHA がそのままコミットの SHA です。

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

各引数は、Attestation が次の条件を満たすことを確かめます。

| 引数 | 確かめること |
|---|---|
| `--repo` | Attestation が GATES のリポジトリで作られたこと |
| `--signer-workflow` | 署名者が、ビルドと Attestation の作成を同じワークフローの中で行う GATES の Reusable Workflow（ほかのワークフローから呼び出されるワークフロー）であること。ビルドしていないワークフローが任意の zip に署名した Attestation を退ける |
| `--signer-digest` | 署名した Reusable Workflow が、ビルドしたコミットのものであること。Reusable Workflow は同じリポジトリから呼ぶため、`--source-digest` と同じコミットになる |
| `--source-ref` | main ブランチで起動したワークフローで作られたこと。リリースのワークフローは main で起動したときだけ動くため、タグではなく main を指定する |
| `--source-digest` | リリースのタグが指すコミットからビルドされたこと |
| `--deny-self-hosted-runners` | Attestation が、GitHub が用意する Runner で作られたこと |

検証が成功すると、コマンドは終了コード 0 で終わります。
どれかの条件に合わないときは、0 以外の終了コードで終わります。
検証に失敗した配布物は、デプロイに使いません。

## 5. Checking the Deployed Function Code

Lambda の関数が実行しているコードは、関数の `CodeSha256` と配布物の SHA-256 を比べて確かめます。
一致すれば、関数は §3 と §4 で来歴を確かめた配布物と同じコードを実行しています。

GATES の Terraform でデプロイしたときも、この照合が使えます。
Terraform が Lambda に上げるのは、**デプロイパッケージ**（`make tf-plan` が手元のソースからビルドし、`make tf-apply` が Lambda に上げる `dist/lambda.zip`）です。
デプロイパッケージは、配布物と同じビルドの設定で、`go.mod` の `toolchain` の行が指すバージョンの Go を使ってビルドします。
手元の Go がそのバージョンでなければ、Go のコマンドがそのバージョンを取得して使います。
そのため、リリースのタグをチェックアウトしてデプロイすれば、デプロイパッケージは配布物と同じバイト列になります。
ただし、次のときは一致しません。

- v0.1.2 以前のリリースのタグからデプロイしたとき。そのタグにある `Makefile` は、配布物と異なる設定でビルドする
- 手元の `zip` コマンドが、配布物のビルド（Ubuntu 24.04 の `zip`）と異なるバイト列を作るとき

`CodeSha256` は、Lambda に上げた zip の SHA-256 を Base64 で表した値です。
`<関数名>` には、デプロイした Lambda の関数の名前を入れます。
GATES の Terraform でデプロイしたときは、`make -C terraform output` が表示する `lambda_function_name` の値です。
`<配布物のパス>` には、§3 でダウンロードした配布物のパスを入れます。

```bash
aws lambda get-function --function-name <関数名> --query Configuration.CodeSha256 --output text
openssl dgst -sha256 -binary <配布物のパス> | openssl base64
```

2 つの出力が一致しないときは、関数に上がっている zip が配布物と異なります。
原因を切り分けるには、デプロイしたリリースのタグをチェックアウトし、`make tf-plan` がデプロイパッケージを作るときと同じ手順（`make lambda-package`）で、デプロイパッケージだけを作り直して配布物と比べます。

```bash
make lambda-package
openssl dgst -sha256 -binary dist/lambda.zip | openssl base64
```

この値が配布物と一致すれば、手元のビルドは配布物を再現できています。
デプロイしたときのソース（別のコミットや、コミットしていない変更）や `zip` コマンドが、今チェックアウトしたタグでのビルドと異なっていたか、Terraform の外で関数のコードが書き換えられた可能性があります。
この値が配布物と一致しなければ、手元のソースか `zip` コマンドが、配布物のビルドと異なります。
コミットしていない変更がないか、v0.1.2 以前のタグでないか、手元の `zip` コマンドが異なるバイト列を作っていないかを確かめます。

## 6. References

- [GitHub — Using artifact attestations to establish provenance for builds](https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/use-artifact-attestations)
- [GitHub CLI — gh attestation verify](https://cli.github.com/manual/gh_attestation_verify)
- [AWS — GetFunction](https://docs.aws.amazon.com/lambda/latest/api/API_GetFunction.html)
- [AWS — FunctionConfiguration](https://docs.aws.amazon.com/lambda/latest/api/API_FunctionConfiguration.html)
