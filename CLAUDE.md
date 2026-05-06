# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## プロジェクト概要

Goで実装された暗号資産自動売買ボット。Bitflyer（メイン）およびOKEX取引所でBTC_JPY/ETH_JPYの買い注文をスケジュール実行し、約定後に+1.5%の売り注文を自動発注する。現物取引のみ対応。

## ディレクトリ構成

- `go/` — Goアプリケーション本体（売買ロジック、API連携、モデル等）
- `terraform/` — AWSインフラ管理（VPC、RDS、EventBridge）
- `db/` — Atlas管理のDBマイグレーション。スキーマ定義は `db/crypto-trading-db/atlas/schema.hcl`
- `lambda/aws-scheduler/` — RDS起動・停止用のLambda関数

## ビルド・実行コマンド（ルートMakefile）

```bash
make run                      # トレーディングアプリ実行（go run）
make run-binary               # ビルド済みバイナリで実行
make run-save-price-history   # 価格履歴記録ジョブ単体実行
make run-send-results         # 日次損益レポートジョブ単体実行
make build                    # バイナリビルド（Linux amd64）→ go/bfTradingApp
```

## DBマイグレーション（db/ディレクトリで実行）

```bash
cd db
make up                       # Atlas導入 + マイグレーション適用
make atlas-status             # マイグレーション状態確認
make atlas-apply              # 未適用マイグレーション適用
make atlas-diff n=<timestamp>_<説明>  # schema.hclの差分からマイグレーション生成
make atlas-hash               # チェックサム更新（手動編集時に必要）
make reset-db                 # ⚠️ DB全削除・再作成
```

DBスキーマ変更の手順: `schema.hcl`を編集 → `make atlas-diff` → 生成SQLを確認 → `make atlas-apply`

## 開発ルール

- **ジョブ関数**: `bitflyerApp/`配下でジョブごとに別ファイルに分離すること
- **エラー処理**: エラーは必ずSlackに通知（`slackClient.PostMessage()`）。OrderID・Price・Size・Strategy等のコンテキストを含めること
- **タイムゾーン**: スケジューラの時刻はシステムTZに依存する。JST対象はUTCに変換（JST = UTC+9）するか、ジョブ内でJSTを明示チェック
- **機密情報**: `private_config.ini`、`terraform.tfvars`は絶対にコミットしないこと

## 設定ファイル

- `go/config.ini` — アプリ設定（取引所、通貨ペア、予算基準、取引量、スケジュール）
- `go/private_config.ini` — APIキー、DB接続情報、Slack webhook（`[sample]private_config.ini`からコピー）
- `db/envs/.db.env` — Atlasマイグレーション用DB接続情報

## テスト

現時点でテストコードは存在しない。
