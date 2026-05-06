# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this directory.

## 概要

AWS上の暗号資産トレーディングボット用インフラをTerraformで管理する。

## 構成

```
terraform/
├── main.tf              # メイン設定（プロバイダ、バックエンド、モジュール呼び出し）
├── terraform.tfvars     # 変数値（機密情報含む、コミット禁止）
└── modules/
    ├── network/         # VPC、サブネット、ルートテーブル
    ├── database/        # RDS MySQL 8.4.7インスタンス
    └── scheduler/       # EventBridgeによるRDS起動・停止スケジューラ
```

## コマンド（ルートディレクトリのMakefileを使用）

```bash
make plan    # 変更内容の確認
make apply   # インフラ変更の適用
make fmt     # Terraformコードのフォーマット
make init    # Terraform初期化
```

**重要**: `terraform`コマンドを直接使わず、必ず`make`コマンドを使うこと。MakefileがAWSプロファイルとバージョン管理を行う。

## 設定情報

- AWSプロファイル: `crypto-trading-20251113`
- リージョン: `ap-northeast-1`（東京）
- Terraformバージョン: `1.13.5`（tfenvで管理）
- 状態管理: S3バックエンド（`tfstate-crypto-trading-20251113`）

## RDSスケジュール

EventBridgeでRDSインスタンスを自動起動・停止:
- **起動**: 3:00 JST（18:00 UTC前日）、16:30 JST（7:30 UTC）
- **停止**: 12:30 JST（3:30 UTC）、23:00 JST（14:00 UTC）

トレーディング稼働時間: 3:00〜12:30、16:30〜23:00（JST）

## 開発ルール

- Terraform修正後は必ず`make plan`でエラーがないことを確認すること
- `terraform.tfvars`は機密情報を含むため絶対にコミットしないこと（`.gitignore`に含まれている）
