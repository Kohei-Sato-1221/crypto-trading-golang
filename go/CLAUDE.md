# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this directory.

## アーキテクチャ

エントリポイントは `cmds/` 配下:
- `cmds/bifflyer_trading/main.go` — メインのトレーディングボット（スケジュールジョブ起動）
- `cmds/save_price_history_job/main.go` — 価格履歴記録の単体実行
- `cmds/send_results_job/main.go` — 日次損益レポートの単体実行

## パッケージ構成

- `app/bitflyerApp/` — 売買ロジックの中核。買い注文・売り注文・約定チェック・注文同期・ジョブスケジューリング。ジョブごとにファイル分離（`placeBuyOrder.go`、`placeSellOrder.go`、`savePriceHistoryJob.go`、`sendResultJob.go`等）
- `bitflyer/`、`okex/`、`bitbank/` — 各取引所のAPIクライアント（HMAC-SHA256認証）
- `models/` — GORMモデル + 生SQL（MySQL）。テーブル: buy_orders、sell_orders、price_histories
- `config/` — INI形式の設定管理（`config.ini`が公開設定、`private_config.ini`が機密設定）
- `slack/` — Slack通知（注文通知、エラーアラート、日次レポート）
- `enums/` — 取引戦略（StrategyLTP99/98/95、加重平均戦略等）と曜日マッピング
- `utils/` — 価格計算、ロギング、日時ヘルパー

## スケジュールジョブ

- **買い注文**: 曜日ごとに異なる戦略で`config.ini`の`trigger_times`に従い実行
- **売り注文**: 180秒ごとに約定済み買い注文をチェックして発注
- **注文同期**: 90秒ごとに取引所APIと同期
- **約定チェック**: 90秒ごとに注文ステータス確認
- **価格履歴**: 毎日6:00/18:00にBTC・ETH価格を記録
- **日次レポート**: 6:45に損益サマリーをSlack送信
- **注文キャンセル**: 23:45に未約定注文をキャンセル

## 主要な依存ライブラリ

- `carlescere/scheduler` — cronライクなジョブスケジューリング
- `gopkg.in/ini.v1` — INI設定ファイルパーサー
- `gorm.io/gorm` + `gorm.io/driver/mysql` — ORM
- Go 1.24.4

## 開発ルール

- ジョブ関数は`bitflyerApp/`配下で必ず別ファイルに分離すること
- エラーは必ずSlackに通知し、コンテキスト（OrderID、Price、Size、Strategy）を含めること
- タイムゾーン: スケジューラはシステムTZに依存。JST対象はUTC変換するかジョブ内でJSTチェック
- `private_config.ini`はコミット禁止
