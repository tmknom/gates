# Key Material Import

運用者に向けて、GitHub App Private Key を生成し、KMS Key に鍵素材としてインポートする手順を示します。
初めてのデプロイと鍵の更新の両方で、この手順を使います。

## 1. Running the Import

始める前に、次を満たしておきます。

- インポート先の KMS Key が `terraform/local/terraform.tfvars` の `kms_keys` にあり、`make tf-apply` で作られている
- AWS の一時認証情報がシェルにある
- AWS CLI、`jq`、`xxd`、OpenSSL 3 以降がある。macOS 標準の LibreSSL では動かない

GitHub App の設定ページで、`Credentials` → `Key pairs` → `New key` をクリックし、秘密鍵をダウンロードします。
秘密鍵が手元にある時間を短くするため、すぐにリポジトリのルートで次のコマンドを実行します。
`KMS_KEY_NAME` には、`kms_keys` でのインポート先の名前を指定します。

```bash
make kms-import-key-material KMS_KEY_NAME=key-2 GITHUB_PRIVATE_KEY_PATH=<ダウンロードした秘密鍵のパス>
```

最後に、KMS Key の状態と公開鍵のフィンガープリントが出ます。

```text
KMS key:     key-2 arn:aws:kms:...
Key state:   Enabled KEY_MATERIAL_DOES_NOT_EXPIRE
Fingerprint: SHA256:...
```

`Fingerprint` が、GitHub App の `Key pairs` に新しく加わった鍵のフィンガープリントと一致すれば成功です。

## 2. What the Command Does

- インポート先の KMS Key が、鍵素材を持たない署名用の鍵（`PendingImport`）であることを確かめる。鍵素材をインポート済みの KMS Key には、インポートしない
- 秘密鍵の鍵長が、KMS Key の鍵の種類（`key_spec`）と一致することを確かめる
- 鍵素材を暗号化してインポートする。鍵素材は無期限で使える
- **ダウンロードした秘密鍵を削除する**。途中で止まったときや、Ctrl-C で中断したときも削除する
- インポートした KMS Key の公開鍵が、秘密鍵のものと一致することを確かめる

## 3. When the Import Stops

止まったときの扱いは、どこで止まったかで決まります。

| 止まった場所 | 秘密鍵のファイル | 扱い |
|---|---|---|
| 秘密鍵を読む前（KMS Key の名前や状態、ツールの不足など） | 残る | 原因を直して、もう一度実行する |
| 秘密鍵を読んだ後、インポートの前 | 削除される | 原因を直し、新しい鍵を生成して実行し直す。使えなくなった鍵は、GitHub App の `Key pairs` から削除する |
| 鍵長が `key_spec` と違う | 削除される | その KMS Key は使えない。鍵長に合わせた `key_spec` の KMS Key を `kms_keys` に加えて適用し、その名前で実行し直す。使えなくなった KMS Key は、Terraform の管理から外して削除を予約する |
| インポートの後（状態やフィンガープリントが違う） | 削除される | 鍵素材は取り込まれている。その KMS Key を使用中の鍵にせず、原因を調べる |

GitHub は GitHub App の最後の鍵を削除させないため、使えなくなった鍵は、新しい鍵を生成した後に削除します。
`kms_keys` に置ける KMS Key は 2 つまでのため、すでに 2 つあるときは、使えなくなった KMS Key を先に外してから加えます。

秘密鍵として読めないファイルは、パスの打ち間違いで関係のないファイルを消さないよう、削除しません。
シンボリックリンクは、リンク先の秘密鍵を消せないため、受け付けません。

## 4. References

- [GitHub — Managing private keys for GitHub Apps](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/managing-private-keys-for-github-apps)
- [AWS — Importing key material for AWS KMS keys](https://docs.aws.amazon.com/kms/latest/developerguide/importing-keys.html)
