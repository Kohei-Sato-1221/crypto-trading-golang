# タスクチェックリスト: Supabase PostgreSQL 移行

## 進め方ルール

- **実装エージェント**: コードを実装する
- **レビュワーエージェント**: 実装完了後にレビューを行う
- レビュー指摘あり → ユーザーが判断 → 修正 → コミット
- レビュー指摘なし → そのままコミットして次のタスクへ

---

## タスク一覧

### Phase 1: DB基盤の構築

- [ ] 1-1. `go/database/interface.go` 作成（DBClient interface定義）
- [ ] 1-2. `go/database/postgres_client.go` 作成（PostgreSQL接続実装）
- [ ] 1-3. `go/database/mysql_client.go` 作成（既存MySQL接続をinterface準拠に移植）
- [ ] 1-4. `go/config/config.go` 修正（driver/postgres DSN設定追加）
- [ ] 1-5. `go/models/database.go` 作成（interface経由でDB初期化するエントリポイント）

### Phase 2: クエリのPostgreSQL対応

- [ ] 2-1. `go/models/events.go` のrawクエリをPostgreSQL対応（プレースホルダ、DATE_FORMAT→TO_CHAR）
- [ ] 2-2. `go/models/price_history.go` のrawクエリをPostgreSQL対応
- [ ] 2-3. `go/okex/events.go` のrawクエリをPostgreSQL対応
- [ ] 2-4. 各`cmds/*/main.go` のDB初期化呼び出しを新interface経由に変更

### Phase 3: docker-compose & マイグレーション

- [ ] 3-1. `docker-compose.yml` 作成（PostgreSQL 16）
- [ ] 3-2. `db/crypto-trading-db-postgres/` 作成（PostgreSQL用スキーマ・初期化SQL）
- [ ] 3-3. `db/Makefile` にPostgreSQL用コマンド追加

### Phase 4: ユニットテスト

- [ ] 4-1. `go/tests/helpers_test.go` 作成（テストDB接続・クリーンアップヘルパー）
- [ ] 4-2. `go/tests/integration_test.go` 作成（DB基盤テスト: 接続、INSERT、UPDATE）
- [ ] 4-3. `go/tests/price_history_test.go` 作成（save_price_history_job テスト）
- [ ] 4-4. `go/tests/send_results_test.go` 作成（send_results_job テスト）
- [ ] 4-5. テスト用設定ファイル作成（test_config.ini, test_private_config.ini）
- [ ] 4-6. ルート`Makefile` に `make test` / `make test-keep-db` 追加

### Phase 5: データ移行ツール

- [ ] 5-1. `scripts/migration/dump_mysql.sh` 作成
- [ ] 5-2. `scripts/migration/import_to_supabase.sh` + `create_tables.sql` 作成
- [ ] 5-3. `scripts/migration/reset_supabase.sh` 作成
- [ ] 5-4. ルート`Makefile` に `make migrate-dump/import/reset` 追加
- [ ] 5-5. `.gitignore` に `dump/` 追加

### Phase 6: 最終確認

- [ ] 6-1. `go.mod` / `go.sum` 更新（postgres driver追加）
- [ ] 6-2. `make test` で全テストパス確認
- [ ] 6-3. 既存MySQL設定でもビルドが通ることの確認
