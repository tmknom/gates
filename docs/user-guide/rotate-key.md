# Key Rotation

運用者に向けて、GitHub App Private Key を新しい鍵に替える鍵の更新の手順を示します。
インポートした鍵素材は入れ替えられないため、新しい KMS Key を加えて新しい鍵をインポートし、使用中の鍵を切り替えてから、旧 KMS Key を退役させます。
発行は止まらず、ワークフローと接続情報も変わりません。

## 1. Order of the Work

例では、旧鍵の名前を `key-1`、新しい鍵の名前を `key-2` とします。
新しい鍵には、使ったことのない次の番号を付けます。

| 順 | 操作 | 切り戻し |
|---|---|---|
| 1 | 新しい KMS Key を加える（§3） | 要らない |
| 2 | 新しい秘密鍵を生成してインポートする（§4） | 要らない |
| 3 | 使用中の鍵を新しい KMS Key に切り替える（§5） | `active_kms_key` を旧鍵に戻して適用する |
| 4 | 新しい鍵での発行を確かめる（§6） | 確認だけ |
| 5 | 旧 KMS Key を無効にする（§7） | `aws kms enable-key` で有効に戻す |
| 6 | GitHub App から旧秘密鍵を削除する（§8） | **切り戻せない** |
| 7 | 削除の後の発行を確かめる（§9） | 確認だけ |
| 8 | 旧 KMS Key を退役させる（§10） | 7 日のあいだは `aws kms cancel-key-deletion` で取り消せる |

**順 6 は、順 4 と順 5 の確認がそろってから行います。**
順 6 の後に問題が見つかったら、次の番号の鍵でこの手順をもう一度行います。
数日様子を見てから旧秘密鍵を削除したいときは、順 5 と順 6 のあいだに期間を置きます。

## 2. Prerequisites

- `kms_keys` に旧鍵だけがあり、`active_kms_key` が旧鍵を指している
- `make tf-plan` が `No changes` を示す
- 許可オーナーのリポジトリで、GATES からトークンを受け取るワークフローが成功する
- [Deployment](deployment.md) の §2.1 と §2.2 の前提を満たしている
- 一時認証情報に、`kms:GetPublicKey`、`kms:DisableKey`、`kms:EnableKey`、`kms:ScheduleKeyDeletion`、`kms:CancelKeyDeletion`、`cloudtrail:LookupEvents` の権限もある

コマンドは、リポジトリのルートで同じシェルから実行します。
旧鍵の ARN を環境変数に設定しておきます。

```bash
export OLD_KMS_KEY_ARN="$(jq -r '.outputs.kms_key_arns.value["key-1"]' terraform/local/terraform.tfstate)"
```

各段階で `make tf-apply` が失敗したら、`make tf-plan` からやり直します。

## 3. Adding the New KMS Key

`terraform/local/terraform.tfvars` の `kms_keys` に新しい鍵を加えます。
鍵の種類は、旧鍵と同じにします。
`active_kms_key` は旧鍵のままにします。

```hcl
kms_keys = {
  key-1 = "RSA_2048"
  key-2 = "RSA_2048"
}
active_kms_key = "key-1"
```

Plan が `aws_kms_external_key.signing["key-2"]` の作成と、Output の `kms_key_arns` の変更だけを示すことを確かめてから適用します。
適用の後の `make tf-plan` が `No changes` を示すことを確かめます。

```bash
make tf-plan
make tf-apply
make tf-plan
```

新しい鍵の ARN を環境変数に設定し、状態が `PendingImport` であることを確かめます。

```bash
export NEW_KMS_KEY_ARN="$(jq -r '.outputs.kms_key_arns.value["key-2"]' terraform/local/terraform.tfstate)"
aws kms describe-key --key-id "$NEW_KMS_KEY_ARN" \
  --query 'KeyMetadata.[KeyState,Origin,KeyUsage,KeySpec]' --output text
```

## 4. Importing the New Private Key

[Key Material Import](import-key-material.md) の手順で、新しい秘密鍵を生成して `key-2` にインポートします。

```bash
make kms-import-key-material KMS_KEY_NAME=key-2 GITHUB_PRIVATE_KEY_PATH=<ダウンロードした秘密鍵のパス>
```

出力の `Fingerprint` が、`Key pairs` に新しく加わった鍵と一致することを確かめます。
インポートの後の `make tf-plan` が `No changes` を示すことを確かめます。

鍵長が `key_spec` と違って止まったときは、GitHub App から生成した鍵を削除し、`key-2` を §10 の手順で退役させます。
そのとき、`kms_keys` から除くのは `key-2`、`active_kms_key` は `key-1` のままにし、`KMS_KEY_NAME=key-2` と `NEW_KMS_KEY_ARN` を使います。
`key-2` は署名に使えないため、§7 の無効化は要りません。
続けて、鍵長に合わせた鍵の種類で、`key-3` を §3 から加えます。

## 5. Switching the Active Key

`active_kms_key` を新しい鍵にします。

```hcl
active_kms_key = "key-2"
```

Plan が次の変更だけを示すことを確かめてから適用し、適用の後の `make tf-plan` が `No changes` を示すことを確かめます。

- `aws_lambda_function.gates` の環境変数 `GATES_KMS_KEY_ARN` が新しい鍵の ARN に変わる
- `aws_iam_policy.lambda_sign` の `Resource` が新しい鍵の ARN に変わる
- Output の `kms_key_arn` が新しい鍵の ARN に変わる

```bash
make tf-plan
make tf-apply
make tf-plan
```

## 6. Checking Issuance with the New Key

§2 のワークフローを実行し、成功することを確かめます。
続けて、その時間帯の KMS の署名の記録を CloudTrail から引き、署名に使われた鍵がすべて新しい鍵であることを確かめます。
`<開始>` と `<終了>` は `YYYY-MM-DDThh:mm:ssZ` の形の時刻です。

```bash
aws cloudtrail lookup-events --region <KMS Key のリージョン> \
  --lookup-attributes AttributeKey=EventName,AttributeValue=Sign \
  --start-time <開始> --end-time <終了> \
  --query 'Events[].CloudTrailEvent' --output json \
  | jq -r '.[] | fromjson | [.eventTime, .resources[0].ARN] | @tsv'
```

発行が失敗するか、旧鍵の ARN が残っていたら、先へ進みません。
`active_kms_key` を旧鍵に戻して適用し、原因を調べます。

## 7. Disabling the Old KMS Key

GitHub の鍵を見分けるため、旧鍵の公開鍵のフィンガープリントを控えます。
出力は、`Key pairs` に表示される値の `SHA256:` の後の部分にあたります。

```bash
aws kms get-public-key --key-id "$OLD_KMS_KEY_ARN" --query PublicKey --output text | openssl base64 -d -A | openssl sha256 -binary | openssl base64
```

旧鍵を無効にし、状態が `Disabled` であることを確かめます。

```bash
aws kms disable-key --key-id "$OLD_KMS_KEY_ARN"
aws kms describe-key --key-id "$OLD_KMS_KEY_ARN" --query 'KeyMetadata.KeyState' --output text
```

`make tf-plan` が `No changes` を示し、§2 のワークフローが成功することを確かめます。
失敗したら、`aws kms enable-key --key-id "$OLD_KMS_KEY_ARN"` で旧鍵を有効に戻し、§6 の切り戻しを行います。

## 8. Deleting the Old Private Key from GitHub

**この操作の後は、旧鍵に切り戻せません。**

GitHub App の設定ページで `Credentials` → `Key pairs` を開き、§7 で控えたフィンガープリントの鍵を `Delete` します。
残った鍵のフィンガープリントが、新しい鍵のものと一致することを確かめます。

```bash
aws kms get-public-key --key-id "$NEW_KMS_KEY_ARN" --query PublicKey --output text | openssl base64 -d -A | openssl sha256 -binary | openssl base64
```

## 9. Checking Issuance after Deleting the Old Private Key

§2 のワークフローをもう一度実行し、成功することを確かめます。

## 10. Retiring the Old KMS Key

KMS Key には、Terraform による削除を拒否する設定（`prevent_destroy`）があります。
そのため、旧鍵は Terraform に削除させず、State から外してから削除を予約します。

`kms_keys` から旧鍵を除きます。

```hcl
kms_keys = {
  key-2 = "RSA_2048"
}
active_kms_key = "key-2"
```

旧鍵を State から外します。
外す前の State は `terraform/local/terraform.tfstate.key-1.backup` に残ります。

```bash
make -C terraform state-rm-kms-key KMS_KEY_NAME=key-1
```

Plan が、リソースを変えず、Output の `kms_key_arns` から旧鍵が消える変更だけを示すことを確かめてから適用します。
適用の後の `make tf-plan` が `No changes` を示すことを確かめます。

```bash
make tf-plan
make tf-apply
make tf-plan
```

Plan が `Instance cannot be destroyed` で止まったら旧鍵を State から外しておらず、旧鍵の作成が現れたら `kms_keys` から除いていません。
どちらの場合も適用しません。

最後に、旧鍵の削除を予約し、状態が `PendingDeletion` であることを確かめます。

```bash
aws kms schedule-key-deletion --key-id "$OLD_KMS_KEY_ARN" --pending-window-in-days 7
aws kms describe-key --key-id "$OLD_KMS_KEY_ARN" --query 'KeyMetadata.KeyState' --output text
```

7 日を過ぎると、KMS は旧鍵を削除します。
State のバックアップは、旧鍵の削除が終われば要りません。

## 11. References

- [GitHub — Managing private keys for GitHub Apps](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/managing-private-keys-for-github-apps)
- [AWS — Deleting AWS KMS keys](https://docs.aws.amazon.com/kms/latest/developerguide/deleting-keys.html)
- [AWS — CloudTrail log entry for Sign](https://docs.aws.amazon.com/kms/latest/developerguide/ct-sign.html)
- [Terraform — Remove a resource from state](https://developer.hashicorp.com/terraform/language/state/remove)
