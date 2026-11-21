# Release Verification

運用者に向けて、GitHub Releases の配布物が GATES のリリースのワークフローでビルドされたものかを確かめる手順と、デプロイした関数のコードが配布物と同じかを確かめる手順を示します。
配布物を信頼してよいかは、運用者自身が Attestation で確かめます。

## 1. Release Assets

| ファイル | 内容 |
|---|---|
| `gates-lambda-linux-arm64.zip` | 配布物。Lambda の関数のコード |
| `SHA256SUMS` | 配布物の SHA-256 |

## 2. Checking the Checksum

配布物をダウンロードし、`SHA256SUMS` と照合します。
`<バージョン>` には、`vX.Y.Z` の形のタグを入れます。
GitHub CLI にログインしておきます。

```bash
gh release download <バージョン> --repo tmknom/gates
sha256sum -c SHA256SUMS        # macOS では shasum -a 256 -c SHA256SUMS
```

配布物の行に `OK` と出れば一致です。
この照合で分かるのはダウンロード中の破損だけで、改ざんは §3 で確かめます。

## 3. Verifying the Attestation

リリースのタグが指すコミットの SHA を調べます。
出力の 1 列目が、そのコミットの SHA です。

```bash
git ls-remote https://github.com/tmknom/gates.git refs/tags/<バージョン>
```

`<コミット>` にその SHA を入れて、Attestation を検証します。

```bash
gh attestation verify gates-lambda-linux-arm64.zip \
  --repo tmknom/gates \
  --signer-workflow tmknom/gates/.github/workflows/reusable-build.yml \
  --signer-digest <コミット> \
  --source-ref refs/heads/main \
  --source-digest <コミット> \
  --deny-self-hosted-runners
```

このコマンドは、配布物が GATES のリポジトリのビルド用のワークフローで、リリースのタグのコミットから、GitHub がホストする Runner でビルドされたことを確かめます。
終了コードが 0 なら成功です。
**検証に失敗した配布物は、デプロイに使いません。**

## 4. Checking the Deployed Function Code

Lambda の関数の `CodeSha256` と、配布物の SHA-256 を比べます。
`<関数名>` は `make -C terraform output` の `lambda_function_name` の値です。

```bash
aws lambda get-function --function-name <関数名> --query Configuration.CodeSha256 --output text
openssl dgst -sha256 -binary <配布物のパス> | openssl base64
```

2 つの出力が一致すれば、関数は §3 で確かめた配布物と同じコードを実行しています。
GATES の Terraform は、手元のソースからデプロイパッケージをビルドして関数に上げます。
リリースのタグからビルドすれば、デプロイパッケージは配布物と同じバイト列になるため、この照合が使えます。

一致しないときは、デプロイしたリリースのタグをチェックアウトし、デプロイパッケージだけを作り直して配布物と比べます。

```bash
make lambda-package
openssl dgst -sha256 -binary dist/lambda.zip | openssl base64
```

| 作り直した値と配布物 | 考えられる原因 |
|---|---|
| 一致する | デプロイしたときのソースか `zip` コマンドが、いまと違っていた（別のコミットや、コミットしていない変更）。または、Terraform の外で関数のコードが書き換えられた |
| 一致しない | 手元のソースか `zip` コマンドが、配布物のビルド（Ubuntu 24.04 の `zip`）と違う。v0.1.2 以前のタグは、配布物と異なる設定でビルドするため一致しない |

## 5. References

- [GitHub — Using artifact attestations to establish provenance for builds](https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/use-artifact-attestations)
- [GitHub CLI — gh attestation verify](https://cli.github.com/manual/gh_attestation_verify)
