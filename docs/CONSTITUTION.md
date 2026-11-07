# Constitution

**GitHub App Token Exchange System**

## Purpose

GATES は、GitHub Actions のワークフローが、GitHub App Private Key を保持することなく、必要な GitHub のアクセス権限を安全かつ簡単に取得できるようにするシステムです。

GitHub App Private Key の保護とアクセス権限の委譲を分離することで、GitHub Actions によるリポジトリを横断した安全な自動化を支えます。

## Problem Statement

GitHub Actions の `GITHUB_TOKEN` には、ワークフローが実行されるリポジトリの外部へアクセスする際の制約があります。

GitHub App を利用すればリポジトリを横断したアクセスが可能になりますが、ワークフローに GitHub App Private Key を配布すると、その漏えいによって GitHub App の広範な権限が悪用されるリスクが生じます。
また、GitHub App Private Key の配布・保護・更新をワークフローごとに管理することは、運用負担の増加にもつながります。

ワークフローへ GitHub App Private Key を配布せず、正当な要求に対して必要な範囲のアクセス権限だけを提供する仕組みが必要です。

## Value Proposition

GATES は、ワークフロー利用者が GitHub App Private Key の管理や Installation Access Token を発行する内部処理を意識せず、必要なアクセス権限をセルフサービスで取得できる環境を提供します。

GATES 運用者は、GitHub App Private Key の保護と Installation Access Token の発行の制御を集約し、個々のワークフローへ GitHub App Private Key を配布する必要をなくせます。

## Design Principles

### 1. 安全性を最優先する

安全性と利便性が対立する場合は、原則として安全性を優先します。

リスクとその影響を適切に評価し、許容できる範囲で安全性を確保することを重視します。
リスクを完全になくすことは目指しません。
安全性を犠牲にする判断には、具体的な必要性とリスクを許容する理由を求めます。

認証・認可などの安全性に関わる条件を確認できない場合は、推測によってアクセスを許可しません。

### 2. 強い資格情報をワークフローから分離する

GitHub App Private Key や GitHub App JWT のように、GitHub App の権限全体を行使できる資格情報を **強い資格情報** と呼びます。
強い資格情報は、ワークフローへ配布しません。

強い資格情報は保護された環境で管理し、ワークフロー利用者には必要な対象と権限に限定された短命な Installation Access Token を提供します。

### 3. 明示的な認可と最小権限を追求する

ワークフローの認証が成功したことだけを理由に、Installation Access Token の発行を許可しません。

発行する Installation Access Token の対象と権限だけでなく、呼び出し元が要求できる範囲も含めて、必要以上のアクセス権限を与えないことを目指します。

認可の粒度は、権限の影響範囲と残留リスクに応じて判断します。
利便性のために、対象や権限を暗黙に拡大しません。

### 4. 機能の豊富さより実装品質を優先する

GATES は、機能の豊富さよりも、認証・認可・資格情報保護などの中核機構の正確性と信頼性を重視します。

機能の追加によって仕様やコードが複雑になると、脆弱性や検証漏れのリスクが増加します。
そのため、機能を必要な範囲に絞り込み、中核機構を十分に理解・検証できる状態を維持します。

機能を拡張する場合は、提供価値だけでなく、安全性と実装品質への影響を評価します。

### 5. ワークフロー利用者の操作を単純に保つ

ワークフロー利用者が意識するのは、必要なアクセス権限を取得するための最小限の操作です。

認証や Installation Access Token を発行する内部処理は GATES 側に集約し、内部実装の変更がワークフロー利用者のワークフローへ波及しない境界を維持します。

ただし、操作の簡略化によって安全性や権限の明確さを損ないません。

## Non-Goals

### GitHub Actions 以外への対応

GATES は、GitHub Actions 以外の実行環境や外部クライアントへの Installation Access Token の発行を目的としません。

### 汎用的な資格情報管理

GATES は、任意のシステムの秘密情報を管理・配布する汎用的な資格情報管理基盤を目指しません。

### GitHub 側の設定と操作

GATES は、GitHub App のインストール先や権限の設定、旧 GitHub App Private Key の失効など、GitHub 側の設定と操作を管理・代行しません。

### ワークフローの処理

GATES は、発行した Installation Access Token を利用する GitHub 上の操作や、ワークフローそのものの実行を代行しません。
