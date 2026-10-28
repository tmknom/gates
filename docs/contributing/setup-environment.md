# Testing Environment Setup

GATES の動作を検証するには、トークンの発行元となる GitHub App と、その Private Key が必要です。
本ドキュメントでは、検証用の GitHub App を個人アカウントに作成し、インストールして、Private Key を発行するまでを扱います。

## 1. GitHub App

### 1.1 Creating the GitHub App

`Settings` → `Developer settings` → `GitHub Apps` → `New GitHub App` の順に開き、GitHub App の作成ページで次のように設定します。

| 項目 | 設定値 |
|---|---|
| GitHub App name | `Testing GATES` |
| Description | `A GitHub App for testing GATES (GitHub App Token Exchange System), a system that issues GitHub App installation access tokens to GitHub Actions workflows.` |
| Homepage URL | `https://example.com` |
| Webhook | `Active` のチェックを外す |
| Repository permissions | `Contents` を `Read-only` にする |
| Account permissions | 何も選択しない |
| Where can this GitHub App be installed? | `Only on this account` を選ぶ |

最後に `Create GitHub App` をクリックすると、GitHub App が作成されます。

> [!NOTE]
> `Contents` を `Read-only` にすると、`Metadata` も自動的に `Read-only` になり、`Mandatory` ラベルが付きます。

### 1.2 Installing the GitHub App

`Settings` → `Developer settings` → `GitHub Apps` の順に開き、作成した GitHub App の `Edit` から設定ページを開きます。
左のメニューの `Install App` を開き、自分のアカウントの `Install` をクリックします。

インストール先のリポジトリは `Only select repositories` を選び、`Select repositories` で検証に使うリポジトリだけを追加します。
最後に `Install` をクリックすると、インストールが完了します。

## 2. GitHub App Private Key

GitHub App の設定ページ（[1.2](#12-installing-the-github-app) と同じページ）で、`Credentials` → `Key pairs` → `New key` をクリックします。
`<app-name>.<date>.private-key.pem` という名前のファイルがダウンロードされるので、リポジトリの外のディレクトリに保存します。

ダウンロードした鍵が正しいかは、次のコマンドで出力される SHA256 フィンガープリントを、設定ページに表示されたフィンガープリントと比べて確かめます。

```bash
openssl rsa -in /path/to/private-key.pem -pubout -outform DER \
  | openssl sha256 -binary | openssl base64
```

## 3. Sources

- [GitHub — Registering a GitHub App](https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/registering-a-github-app)
- [GitHub — Installing your own GitHub App](https://docs.github.com/en/apps/using-github-apps/installing-your-own-github-app)
- [GitHub — Managing private keys for GitHub Apps](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/managing-private-keys-for-github-apps)
