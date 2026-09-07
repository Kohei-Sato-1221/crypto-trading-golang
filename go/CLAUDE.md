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
- `models/` — GORMモデル + 生SQL。本番は **PostgreSQL(Supabase)**（`private_config.ini` の `driver` で切替。MySQL版のクエリも残置）。
  テーブル: buy_orders、sell_orders、price_histories
- `config/` — INI形式の設定管理（`config.ini`が公開設定、`private_config.ini`が機密設定）
- `slack/` — Slack通知（注文通知、エラーアラート、日次レポート）
- `enums/` — 取引戦略（StrategyLTP99/98/95、加重平均戦略等）と曜日マッピング
- `utils/` — 価格計算、ロギング、日時ヘルパー

## スケジュールジョブ

- **買い注文**: 曜日ごとに異なる戦略で `config.ini` の `trigger_time_01`（既定 06:30）に従い実行
- **売り注文**: 180秒ごとに約定済み買い注文をチェックして発注
- **注文同期**: 90秒ごとに取引所APIと同期
- **約定チェック**: 90秒ごとに注文ステータス確認
- **売り注文のローリング**: `trigger_time_05`（既定 05:30）。取引所の30日上限に対し27日でキャンセル→同条件で再発注し、実質無期限化する
- **失効検出**: `trigger_time_06`（既定 06:05）。期限切れ・消滅した注文をCANCELLEDに落としスロットを解放する
- **日次リコンサイル**: `trigger_time_07`（既定 06:15）。取引所とDBの乖離・発注ゼロ・スロット枯渇を検知してSlack通知する
- **価格履歴**: 毎日6:00/18:00にBTC・ETH価格を記録（`trigger_time_04` / `trigger_time_03`）
- **日次レポート**: `trigger_time_02`（既定 06:45）に損益サマリーをSlack送信
- **注文キャンセル**: `trigger_time_08`（既定 22:45）に長期未約定の買い注文をキャンセル
- **グレースフルシャットダウン**: `trigger_time_09`（既定 01:20）。Pi停止(01:30 JST)の10分前に新規ジョブをブロックする

実行環境は Raspberry Pi 上の systemd サービス（`bfTradingApp.service` / `Restart=always`）で原則24時間稼働する。
ただし **Pi 自体が毎日 01:30〜02:45 JST に停止する**ため、この時間帯にジョブを置いてはならない。
全ジョブの登録は `jobRegistry` 経由で行い、登録に失敗したジョブがあれば起動時にSlackへ通知される
（`scheduler.Run()` の戻り値を捨てると、そのジョブは二度と発火しないまま無言で失われるため）。

## 主要な依存ライブラリ

- `carlescere/scheduler` — cronライクなジョブスケジューリング
- `gopkg.in/ini.v1` — INI設定ファイルパーサー
- `gorm.io/gorm` — ORM
- `jackc/pgx/v5` — PostgreSQLドライバ（PgBouncer対応のため simple_protocol モードを使用）
- Go 1.24.4

## 開発ルール

- ジョブ関数は`bitflyerApp/`配下で必ず別ファイルに分離すること
- エラーは必ずSlackに通知し、コンテキスト（OrderID、Price、Size、Strategy）を含めること
- タイムゾーン: スケジューラはシステムTZに依存。JST対象はUTC変換するかジョブ内でJSTチェック
- `private_config.ini`はコミット禁止
