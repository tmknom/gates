# Key Rotation

本ドキュメントでは、GitHub App Private Key を新しい鍵に更新する手順を扱います。
KMS にインポートした鍵素材は別の鍵素材に交換できないため、新しい KMS Key を加えて鍵素材をインポートし、Lambda が署名に使う鍵（使用中の鍵）を切り替え、旧 KMS Key を退役させます。
Terraform での鍵の表し方は [AWS Infrastructure Design](../design/aws-infrastructure.md) の §4.3 にあります。

更新で変わるのは Lambda の環境変数の Key ARN と、実行 Role の IAM ポリシーだけです。
利用者のワークフローと、GitHub に登録した接続情報は変えません。

## 1. Order of the Work

可逆な操作を先に、不可逆な操作を後に進めます。
例では、旧鍵の名前を `key-1`、新しい鍵の名前を `key-2` とします。
新しい鍵には、使ったことのない次の番号を付けます。

| 順 | 操作 | 切り戻し |
|---|---|---|
| 1 | 新しい KMS Key を加える（§3） | 鍵素材がなく署名に使えないため、切り戻しは要らない |
| 2 | 新しい Private Key を生成してインポートする（§4） | GitHub の新しい鍵は削除できる。インポートした鍵素材は交換できない |
| 3 | 使用中の鍵を新しい KMS Key に切り替える（§5） | 使用中の鍵の指定を旧鍵に戻して適用する |
| 4 | 新しい鍵での発行を確かめる（§6） | 確認だけ |
| 5 | 旧 KMS Key を無効にする（§7） | `aws kms enable-key` で有効に戻す |
| 6 | GitHub App から旧 Private Key を削除する（§8） | **切り戻せない** |
| 7 | 旧鍵の削除の後の発行を確かめる（§9） | 確認だけ |
| 8 | 旧 KMS Key を退役させる（§10） | 予約期間（30 日）のあいだは `aws kms cancel-key-deletion` で削除を取り消せる |

GitHub App は Private Key を 25 個まで同時に持て、GitHub の公式資料は、複数の鍵を使って停止せずに鍵を更新することを勧めています。
2 から 6 のあいだは、新旧どちらの鍵で署名した GitHub App JWT も GitHub に受け付けられます。
6 で GitHub から削除した旧鍵は認証に使えなくなる見込みのため、6 の後は旧鍵に切り戻せません。
6 は、4 と 5 の確認がそろうまで行いません。

## 2. Prerequisites

始める前に次を満たしておきます。

- `terraform/local/terraform.tfvars` の `kms_keys` に旧鍵だけがあり、`active_kms_key` が旧鍵を指している
- `make tf-plan` が `No changes` を示す
- [E2E Tests](e2e-tests.md) の `_e2e-success.yml` が成功する
- [Key Material Import](import-key-material.md) の手順を実行できる（AWS CLI、OpenSSL、`jq`、`xxd`）
- [Testing Environment Setup](setup-environment.md) の §3.1 の前提（一時認証情報、IAM OIDC Provider）を満たしている

コマンドはリポジトリのルートで、ひとつのシェルから実行します。
旧鍵の ARN を確かめて、環境変数に設定しておきます。

```bash
make tf-output
export OLD_KMS_KEY_ARN="$(jq -r '.outputs.kms_key_arns.value["key-1"]' terraform/local/terraform.tfstate)"
```

## 3. Adding the New KMS Key

`terraform/local/terraform.tfvars` の `kms_keys` に新しい鍵を加えます。
`active_kms_key` は旧鍵のままにします。
新しい鍵の `key_spec` は、インポートする鍵の見込みの鍵長に合わせます。
GitHub は生成する鍵の鍵長を文書で約束していないため、旧鍵と同じ値を見込みとします。

```hcl
kms_keys = {
  key-1 = "RSA_2048"
  key-2 = "RSA_2048"
}
active_kms_key = "key-1"
```

`make tf-plan` を実行し、Plan が `aws_kms_external_key.signing["key-2"]` の作成と、Output の `kms_key_arns` の変更だけを示すことを確かめます。
旧鍵、Lambda、IAM の変更と削除が含まれていたら、適用しません。

```bash
make tf-plan
make tf-apply
make tf-plan
```

適用の後の `make tf-plan` が `No changes` を示すことを確かめます。
そして、新しい鍵が `PendingImport` で、`Origin` が `EXTERNAL`、`KeyUsage` が `SIGN_VERIFY`、`KeySpec` が `kms_keys` の値であることを確かめます。

```bash
export NEW_KMS_KEY_ARN="$(jq -r '.outputs.kms_key_arns.value["key-2"]' terraform/local/terraform.tfstate)"
aws kms describe-key --key-id "$NEW_KMS_KEY_ARN" \
  --query 'KeyMetadata.[KeyState,Origin,KeyUsage,KeySpec]' --output text
```

## 4. Importing the New Private Key

[Key Material Import](import-key-material.md) の手順で、新しい Private Key を生成して新しい KMS Key にインポートします。
`KMS_KEY_NAME` には新しい鍵の名前（`key-2`）を設定します。
手順の最後で、鍵が `Enabled` になり、公開鍵のフィンガープリントが GitHub App の `Key pairs` に新しく加わった鍵と一致することと、手元の秘密鍵のコピーと作業ディレクトリを削除したことを確かめます。

鍵長が `key_spec` と違うと、`aws kms import-key-material` が失敗します。
その場合は、手元の秘密鍵のコピーを削除し、GitHub App から生成した鍵を削除してから、§10 の手順の名前と ARN をこの KMS Key に読み替えて退役させ、鍵長に合わせた `key_spec` で次の番号の鍵を §3 から加え直します。
`PendingImport` の鍵は署名に使えないため、無効にする手順（§7）は要りません。

インポートの後に `make tf-plan` を実行し、`No changes` を示すことを確かめます。
鍵の状態が `PendingImport` から `Enabled` に変わっても、Terraform は鍵の有効と無効を設定に書かないため、Plan に差分は出ません。

## 5. Switching the Active Key

`terraform/local/terraform.tfvars` の `active_kms_key` を新しい鍵の名前にします。

```hcl
active_kms_key = "key-2"
```

`make tf-plan` を実行し、Plan が次の変更だけを示すことを確かめてから適用します。

- `aws_lambda_function.gates` の環境変数 `GATES_KMS_KEY_ARN` が新しい鍵の ARN に変わる
- `aws_iam_policy.lambda_sign` の `Resource` が新しい鍵の ARN に変わる
- Output の `kms_key_arn` が新しい鍵の ARN に変わる

```bash
make tf-plan
make tf-apply
make tf-plan
```

適用の後の `make tf-plan` が `No changes` を示すことを確かめます。
関数の設定の更新が終わるまでは、呼び出しは更新前の設定（旧鍵）を使います。

## 6. Checking Issuance with the New Key

[E2E Tests](e2e-tests.md) の `_e2e-success.yml` を実行し、成功することを確かめます。
ワークフローの実行の時間帯の KMS の `Sign` の記録を CloudTrail から引き、署名に使われた鍵がすべて新しい鍵であることを確かめます。
`Sign` の記録は `resources` に KMS Key の ARN を持ちます。

```bash
aws cloudtrail lookup-events --region <GATES_AWS_REGION> \
  --lookup-attributes AttributeKey=EventName,AttributeValue=Sign \
  --start-time <開始> --end-time <終了> \
  --query 'Events[].CloudTrailEvent' --output json \
  | jq -r '.[] | fromjson | [.eventTime, .resources[0].ARN] | @tsv'
```

発行が成功しないか、旧鍵の ARN が残っていたら、先へ進みません。
`active_kms_key` を旧鍵の名前に戻し、`make tf-plan` と `make tf-apply` で切り戻してから原因を調べます。

## 7. Disabling the Old KMS Key

旧鍵を無効にする前に、§8 で GitHub の鍵を見分けるために、旧鍵の公開鍵のフィンガープリントを控えます。

```bash
aws kms get-public-key --key-id "$OLD_KMS_KEY_ARN" --query PublicKey --output text | openssl base64 -d -A | openssl sha256 -binary | openssl base64
```

旧鍵を無効にし、状態が `Disabled` であることを確かめます。

```bash
aws kms disable-key --key-id "$OLD_KMS_KEY_ARN"
aws kms describe-key --key-id "$OLD_KMS_KEY_ARN" --query 'KeyMetadata.KeyState' --output text
```

`make tf-plan` が `No changes` を示すことを確かめます。
そして `_e2e-success.yml` をもう一度実行し、成功することを確かめます。
旧鍵を無効にしても発行できれば、GATES が旧鍵に依存していないと分かります。

発行が成功しなければ、`aws kms enable-key --key-id "$OLD_KMS_KEY_ARN"` で旧鍵を有効に戻し、§6 の切り戻しを行います。

## 8. Deleting the Old Private Key from GitHub

この操作の後は、旧鍵に切り戻せません。
§6 と §7 の確認がそろってから行います。

GitHub App の設定ページで `Credentials` → `Key pairs` を開き、§7 で控えたフィンガープリントの鍵の `Delete` をクリックします。
削除の後、`Key pairs` に残った鍵のフィンガープリントが、新しい KMS Key の公開鍵のフィンガープリントと一致することを確かめます。

```bash
aws kms get-public-key --key-id "$NEW_KMS_KEY_ARN" --query PublicKey --output text | openssl base64 -d -A | openssl sha256 -binary | openssl base64
```

## 9. Checking Issuance after Deleting the Old Key

[E2E Tests](e2e-tests.md) のワークフローをすべて実行し、成功することを確かめます。

## 10. Retiring the Old KMS Key

旧鍵を Terraform の管理から外し、削除を予約します。
Terraform に旧鍵を削除させないのは、`prevent_destroy` が、鍵の集合から除いた鍵の削除を拒否するためです。

`terraform/local/terraform.tfvars` の `kms_keys` から旧鍵を除きます。

```hcl
kms_keys = {
  key-2 = "RSA_2048"
}
active_kms_key = "key-2"
```

続けて、旧鍵を State から外します。
このターゲットは、`KMS_KEY_NAME` で指定した鍵だけを、削除せずに State から外し、外す前の State を `terraform/local/terraform.tfstate.key-1.backup` に残します。

```bash
make tf-state-rm-kms-key KMS_KEY_NAME=key-1
```

`make tf-plan` を実行し、Plan がリソースの変更を示さず、Output の `kms_key_arns` から旧鍵が消える変更だけを示すことを確かめます。
旧鍵の削除が現れて `Instance cannot be destroyed` で止まったら、State から外していません。
旧鍵の作成が現れたら、`kms_keys` から旧鍵を除いていません。
どちらの場合も適用しません。

```bash
make tf-plan
make tf-apply
make tf-plan
```

適用は Output を State に保存するだけで、AWS のリソースを変えません。
適用の後の `make tf-plan` が `No changes` を示すことを確かめます。

最後に、旧鍵の削除を予約し、状態が `PendingDeletion` であることを確かめます。
予約期間は、Terraform の設定の `deletion_window_in_days` と同じ 30 日にします。

```bash
aws kms schedule-key-deletion --key-id "$OLD_KMS_KEY_ARN" --pending-window-in-days 30
aws kms describe-key --key-id "$OLD_KMS_KEY_ARN" --query 'KeyMetadata.KeyState' --output text
```

予約期間のあいだは、`aws kms cancel-key-deletion` で削除を取り消せます。
予約期間を過ぎると、KMS は旧鍵を削除し、取り消せなくなります。

## 11. Sources

- [GitHub — Managing private keys for GitHub Apps](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/managing-private-keys-for-github-apps)
- [AWS — Deleting AWS KMS keys](https://docs.aws.amazon.com/kms/latest/developerguide/deleting-keys.html)
- [AWS — Logging AWS KMS API calls with AWS CloudTrail](https://docs.aws.amazon.com/kms/latest/developerguide/logging-using-cloudtrail.html)
- [AWS — CloudTrail log entry for Sign](https://docs.aws.amazon.com/kms/latest/developerguide/ct-sign.html)
- [AWS — Lambda function states](https://docs.aws.amazon.com/lambda/latest/dg/functions-states.html)
- [Terraform — Remove a resource from state](https://developer.hashicorp.com/terraform/language/state/remove)
