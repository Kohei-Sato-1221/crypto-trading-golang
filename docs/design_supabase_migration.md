# 設計書: Supabase PostgreSQL 移行

## 概要

現在MySQL(AWS RDS)で動作しているcrypto-trading-golangアプリを、Supabase PostgreSQLでも動作するように改修する。MySQLに戻す可能性があるため、DBクライアントをinterface化して切り替え可能にする。

---

## 1. DBクライアントのinterface化

### 1.1 設計方針

- `models/` パッケージにDB操作のinterfaceを定義
- MySQL実装とPostgreSQL実装を分離
- configの設定値で切り替え（`private_config.ini` の `[database]` セクション）
- グローバル変数 `AppDB` / `GormDB` をinterface経由に変更

### 1.2 ディレクトリ構成（変更後）

```
go/
├── database/
│   ├── interface.go         # DBClient interface定義
│   ├── mysql_client.go      # MySQL実装（既存ロジック移植）
│   └── postgres_client.go   # PostgreSQL実装
├── models/
│   ├── mysqlBase.go         # ← コメントアウトして残す（参考用）
│   ├── database.go          # 新: DB初期化（interface経由）
│   ├── events.go            # プレースホルダを$1/$2形式に対応
│   └── price_history.go     # 同上
└── ...
```

### 1.3 interface定義

```go
// go/database/interface.go
package database

import (
    "database/sql"
    "gorm.io/gorm"
)

type DBClient interface {
    Connect() error
    GetDB() *sql.DB
    GetGormDB() *gorm.DB
    Close() error
    DriverName() string  // "mysql" or "postgres"
}
```

### 1.4 切り替え方式

`private_config.ini` に以下を追加:

```ini
[database]
# driver = mysql
# mysql = crypto_trading_root:xxx@tcp(host:port)/crypto_trading

driver = postgres
postgres = postgresql://postgres:AbcfbG294QB2hR@db.hastyhkkwedwqchtlsiv.supabase.co:5432/postgres
```

`config.go` に `PostgresDSN` フィールドと `DBDriver` フィールドを追加し、driverの値で初期化するクライアントを切り替える。

### 1.5 SQL互換性の対応

MySQL → PostgreSQL で変換が必要な箇所:

| MySQL | PostgreSQL | 対象箇所 |
|-------|-----------|----------|
| `?` プレースホルダ | `$1, $2, ...` | 全rawクエリ |
| `DATE_FORMAT(x, '%Y-%m-%d')` | `TO_CHAR(x, 'YYYY-MM-DD')` | GetResults(), GetOKexResults() |
| `CURRENT_TIMESTAMP` ON UPDATE | トリガー or アプリ側で更新 | updatetimeカラム |
| `auto_increment` | `SERIAL` / `GENERATED ALWAYS` | DDL |
| `UNION` + `LIMIT` の構文差異 | サブクエリで囲む | GetResults() |

**方針**: rawクエリ部分はdriver判定で分岐させるヘルパーを作る。GORMのクエリ（`Where().Find()`等）はドライバ透過なのでそのまま動作する。

```go
// go/database/query_helper.go
func Placeholder(driver string, index int) string {
    if driver == "postgres" {
        return "$" + strconv.Itoa(index)
    }
    return "?"
}
```

ただし、rawクエリが多く複雑なため、各関数をMySQL用/PostgreSQL用に分岐させる方式を採用する:

```go
func GetResults() (string, error) {
    if database.CurrentDriver() == "postgres" {
        return getResultsPostgres()
    }
    return getResultsMySQL()
}
```

---

## 2. docker-compose（ローカル開発用PostgreSQL）

### 2.1 ファイル構成

```
docker-compose.yml          # プロジェクトルートに配置
```

### 2.2 docker-compose.yml

```yaml
version: "3.8"
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: crypto_trading
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./db/crypto-trading-db-postgres/init:/docker-entrypoint-initdb.d

volumes:
  postgres_data:
```

### 2.3 ローカル接続情報

```
postgresql://postgres:postgres@localhost:5432/crypto_trading
```

---

## 3. Atlasマイグレーション（PostgreSQL用・新規作成）

### 3.1 方針

- 既存の `db/crypto-trading-db/` (MySQL用) はそのまま残す
- PostgreSQL用を `db/crypto-trading-db-postgres/` に新規作成

### 3.2 ディレクトリ構成

```
db/
├── crypto-trading-db/           # 既存MySQL用（変更なし）
│   ├── atlas/
│   │   └── schema.hcl
│   └── migrations/
├── crypto-trading-db-postgres/  # 新規PostgreSQL用
│   ├── atlas/
│   │   └── schema.hcl          # PostgreSQL版スキーマ定義
│   ├── migrations/
│   ├── init/
│   │   └── 001_init.sql        # docker-compose初期化用SQL
│   └── atlas.hcl               # Atlas設定ファイル
├── envs/
│   └── .db.env
└── Makefile                     # PostgreSQL用コマンド追加
```

### 3.3 PostgreSQL版スキーマ（schema.hcl → SQL）

```sql
-- buy_orders
CREATE TABLE buy_orders (
    id SERIAL PRIMARY KEY,
    order_id VARCHAR(50) UNIQUE,
    product_code VARCHAR(50),
    side VARCHAR(20),
    price DOUBLE PRECISION,
    size DOUBLE PRECISION,
    exchange VARCHAR(50),
    status VARCHAR(100) DEFAULT 'UNFILLED',
    strategy SMALLINT NOT NULL DEFAULT 99,
    remarks TEXT,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updatetime TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- sell_orders
CREATE TABLE sell_orders (
    id SERIAL PRIMARY KEY,
    parentid VARCHAR(50),
    order_id VARCHAR(50) UNIQUE,
    product_code VARCHAR(50),
    side VARCHAR(20),
    price DOUBLE PRECISION,
    size DOUBLE PRECISION,
    exchange VARCHAR(50),
    status VARCHAR(100) DEFAULT 'UNFILLED',
    remarks TEXT,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updatetime TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- price_histories
CREATE TABLE price_histories (
    id SERIAL PRIMARY KEY,
    datetime TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    product_code VARCHAR(50) NOT NULL,
    price DOUBLE PRECISION NOT NULL,
    price_ratio_24h DOUBLE PRECISION,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_product_code_datetime ON price_histories (product_code, datetime);

-- okj_buy_orders (OKJ用)
CREATE TABLE okj_buy_orders (
    id SERIAL PRIMARY KEY,
    order_id VARCHAR(50) UNIQUE,
    pair VARCHAR(50),
    price DOUBLE PRECISION,
    size DOUBLE PRECISION,
    exchange VARCHAR(50),
    state INTEGER DEFAULT 0,
    sell_order_id VARCHAR(50),
    sell_order_state VARCHAR(20),
    sell_price DOUBLE PRECISION,
    side VARCHAR(20),
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updatetime TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- updatetime自動更新用トリガー（PostgreSQLにはON UPDATE CURRENT_TIMESTAMPがないため）
CREATE OR REPLACE FUNCTION update_updatetime()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updatetime = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_buy_orders_updatetime BEFORE UPDATE ON buy_orders
    FOR EACH ROW EXECUTE FUNCTION update_updatetime();
CREATE TRIGGER trg_sell_orders_updatetime BEFORE UPDATE ON sell_orders
    FOR EACH ROW EXECUTE FUNCTION update_updatetime();
CREATE TRIGGER trg_okj_buy_orders_updatetime BEFORE UPDATE ON okj_buy_orders
    FOR EACH ROW EXECUTE FUNCTION update_updatetime();
```

---

## 4. ユニットテスト

### 4.1 方針

- docker-composeのPostgreSQLを起動した状態でテスト実行
- 注文の**発注は行わない**（取引所APIコールなし）
- 対象は3つのcmd: `bifflyer_trading`, `save_price_history_job`, `send_results_job`
- DB操作のテストに加え、`save_price_history_job`と`send_results_job`の一連のフロー（DB部分のみ）をテスト

### 4.2 テスト対象

#### A. DB基盤テスト（database パッケージ）

| テストケース | 概要 |
|-------------|------|
| TestDBConnection | PostgreSQL接続が正常に確立できること |
| TestInsertBuyOrder | buy_ordersテーブルにINSERTできること |
| TestInsertSellOrder | sell_ordersテーブルにINSERTできること |
| TestGetUnfilledBuyOrders | UNFILLED状態の注文を取得できること |
| TestUpdateFilledOrder | ステータス更新が正常に動作すること |
| TestShouldPlaceBuyOrder | 注文可否の判定ロジックが動作すること |
| TestGetResults | 収益計算クエリが動作すること（bitflyer用） |

#### B. save_price_history_job テスト

注文操作なし。価格データのDB保存・取得をテストする。

| テストケース | 概要 |
|-------------|------|
| TestSavePriceHistory | 価格履歴をINSERT → 正常に保存されること |
| TestGetPrice24HoursAgo | テストデータ挿入 → 24時間前の価格を正しく取得できること |
| TestGetLowestPriceInPast7Days | テストデータ挿入 → 7日間最低価格を正しく取得できること |
| TestSavePriceHistoryWithRatio | price_ratio_24hを含むレコードが保存できること |
| TestSavePriceHistoryFlow | 実際のジョブフロー再現: 価格保存→24h前取得→ratio計算→再保存 |

#### C. send_results_job テスト

注文操作なし。収益集計クエリのDB動作をテストする。

| テストケース | 概要 |
|-------------|------|
| TestGetResultsEmpty | データなしの場合にエラーにならないこと |
| TestGetResultsWithData | テスト用buy_orders/sell_ordersを挿入→収益計算が正しいこと |
| TestGetResultsProfitCalculation | 既知の値でINSERT→利益額が期待通りか検証 |

### 4.3 テスト構成

```
go/
├── database/
│   ├── interface.go
│   ├── postgres_client.go
│   └── mysql_client.go
├── tests/
│   ├── integration_test.go      # DB基盤テスト
│   ├── price_history_test.go    # save_price_history_job テスト
│   ├── send_results_test.go     # send_results_job テスト
│   ├── helpers_test.go          # テスト用ヘルパー（DB初期化・クリーンアップ）
│   └── testdata/
│       └── seed.sql             # テスト用シードデータ
├── test_config.ini              # テスト用設定
└── test_private_config.ini      # テスト用DB接続情報（docker-compose用）
```

### 4.4 テストヘルパー設計

```go
// go/tests/helpers_test.go
func setupTestDB(t *testing.T) {
    // docker-composeのPostgreSQLに接続
    // テーブルをTRUNCATEしてクリーンな状態にする
}

func teardownTestDB(t *testing.T) {
    // テスト後にデータをクリーンアップ
}

func insertTestBuyOrder(t *testing.T, orderID string, price float64, status string) {
    // テスト用の買い注文を挿入
}

func insertTestSellOrder(t *testing.T, parentID, orderID string, price float64, status string) {
    // テスト用の売り注文を挿入
}

func insertTestPriceHistory(t *testing.T, productCode string, price float64, datetime time.Time) {
    // テスト用の価格履歴を挿入
}
```

### 4.5 テスト用設定

```ini
# go/test_private_config.ini
[database]
driver = postgres
postgres = postgresql://postgres:postgres@localhost:5432/crypto_trading

[bitflyer]
api_key = test
api_secret = test
base_url = https://api.bitflyer.com

[slack]
api_url = http://localhost:9999/dummy
token = test
```

### 4.6 Makefile

```makefile
# ルートMakefileに追加

test: ## run unit tests with docker-compose PostgreSQL
	docker compose up -d postgres
	@echo "Waiting for PostgreSQL to be ready..."
	@until docker compose exec -T postgres pg_isready -U postgres > /dev/null 2>&1; do sleep 1; done
	@echo "PostgreSQL is ready."
	cd go && go test ./tests/ -v -count=1
	docker compose down

test-keep-db: ## run tests without stopping PostgreSQL (for repeated runs)
	cd go && go test ./tests/ -v -count=1
```

---

## 5. MySQLデータのSupabase移行（ツール化）

### 5.1 方針

何度もやり直す可能性があるため、以下の3つの操作をMakefileコマンドとして独立させる:

| コマンド | 用途 |
|---------|------|
| `make migrate-dump` | MySQLからデータをダンプ |
| `make migrate-import` | ダンプデータをSupabase PostgreSQLにインポート |
| `make migrate-reset` | SupabaseのDBを全削除（テーブルDROP＋再作成）してやり直し可能にする |

### 5.2 ディレクトリ構成

```
scripts/
├── migration/
│   ├── dump_mysql.sh           # MySQLダンプ実行
│   ├── convert_to_postgres.sh  # MySQL SQL → PostgreSQL SQL変換
│   ├── import_to_supabase.sh   # Supabaseへインポート
│   ├── reset_supabase.sh       # Supabaseのテーブル全削除＋再作成
│   └── create_tables.sql       # PostgreSQLテーブル作成DDL
dump/
│   ├── mysql_data.sql          # ダンプ出力先（gitignore）
│   └── postgres_data.sql       # 変換後データ（gitignore）
```

### 5.3 `make migrate-dump` — MySQLからダンプ

```bash
#!/bin/bash
# scripts/migration/dump_mysql.sh
set -e

DUMP_DIR="dump"
mkdir -p $DUMP_DIR

echo "=== Dumping MySQL data from AWS RDS ==="
mysqldump -h crypto-trading-db.cva64ye44jkh.ap-northeast-1.rds.amazonaws.com \
  -P 1221 -u crypto_trading_root -p'CryptoSl0S&mDGdY098' \
  crypto_trading buy_orders sell_orders price_histories okj_buy_orders \
  --no-create-info --complete-insert --skip-lock-tables \
  > $DUMP_DIR/mysql_data.sql

echo "=== Dump complete: $DUMP_DIR/mysql_data.sql ==="
echo "=== Converting to PostgreSQL format ==="

# MySQL → PostgreSQL変換
cat $DUMP_DIR/mysql_data.sql \
  | sed 's/`//g' \
  | sed 's/\\'\''/'\'\''/g' \
  | sed '/^\/\*!/d' \
  | sed '/^--/d' \
  | sed '/^LOCK TABLES/d' \
  | sed '/^UNLOCK TABLES/d' \
  | sed '/^SET /d' \
  > $DUMP_DIR/postgres_data.sql

echo "=== Conversion complete: $DUMP_DIR/postgres_data.sql ==="
```

### 5.4 `make migrate-import` — Supabaseへインポート

```bash
#!/bin/bash
# scripts/migration/import_to_supabase.sh
set -e

SUPABASE_DSN="postgresql://postgres:AbcfbG294QB2hR@db.hastyhkkwedwqchtlsiv.supabase.co:5432/postgres"
DUMP_DIR="dump"

if [ ! -f "$DUMP_DIR/postgres_data.sql" ]; then
  echo "ERROR: $DUMP_DIR/postgres_data.sql not found. Run 'make migrate-dump' first."
  exit 1
fi

echo "=== Creating tables (if not exist) ==="
psql "$SUPABASE_DSN" -f scripts/migration/create_tables.sql

echo "=== Importing data to Supabase ==="
psql "$SUPABASE_DSN" -f $DUMP_DIR/postgres_data.sql

echo "=== Resetting sequences ==="
psql "$SUPABASE_DSN" -c "
SELECT setval('buy_orders_id_seq', COALESCE((SELECT MAX(id) FROM buy_orders), 0) + 1, false);
SELECT setval('sell_orders_id_seq', COALESCE((SELECT MAX(id) FROM sell_orders), 0) + 1, false);
SELECT setval('price_histories_id_seq', COALESCE((SELECT MAX(id) FROM price_histories), 0) + 1, false);
SELECT setval('okj_buy_orders_id_seq', COALESCE((SELECT MAX(id) FROM okj_buy_orders), 0) + 1, false);
"

echo "=== Import complete! ==="
```

### 5.5 `make migrate-reset` — Supabase DBリセット

```bash
#!/bin/bash
# scripts/migration/reset_supabase.sh
set -e

SUPABASE_DSN="postgresql://postgres:AbcfbG294QB2hR@db.hastyhkkwedwqchtlsiv.supabase.co:5432/postgres"

echo "=== WARNING: This will DELETE ALL DATA in Supabase ==="
read -p "Are you sure? (yes/no): " confirm
if [ "$confirm" != "yes" ]; then
  echo "Cancelled."
  exit 0
fi

echo "=== Dropping all tables ==="
psql "$SUPABASE_DSN" -c "
DROP TRIGGER IF EXISTS trg_buy_orders_updatetime ON buy_orders;
DROP TRIGGER IF EXISTS trg_sell_orders_updatetime ON sell_orders;
DROP TRIGGER IF EXISTS trg_okj_buy_orders_updatetime ON okj_buy_orders;
DROP FUNCTION IF EXISTS update_updatetime();
DROP TABLE IF EXISTS sell_orders CASCADE;
DROP TABLE IF EXISTS buy_orders CASCADE;
DROP TABLE IF EXISTS price_histories CASCADE;
DROP TABLE IF EXISTS okj_buy_orders CASCADE;
"

echo "=== All tables dropped. Run 'make migrate-import' to re-import. ==="
```

### 5.6 Makefileコマンド

```makefile
# ルートMakefileに追加
migrate-dump: ## Dump MySQL data from AWS RDS
	bash scripts/migration/dump_mysql.sh

migrate-import: ## Import dumped data to Supabase PostgreSQL
	bash scripts/migration/import_to_supabase.sh

migrate-reset: ## Drop all tables in Supabase (for re-import)
	bash scripts/migration/reset_supabase.sh
```

### 5.7 .gitignore追加

```
dump/
```

---

## 6. 実装順序

| Step | 内容 | 依存 |
|------|------|------|
| 1 | `go/database/` パッケージ作成（interface + PostgreSQL実装） | - |
| 2 | `config.go` にdriver/postgres設定追加 | - |
| 3 | `models/` のクエリをdriver対応に改修 | Step 1, 2 |
| 4 | docker-compose.yml 作成 | - |
| 5 | PostgreSQL用Atlasマイグレーション作成 | - |
| 6 | ユニットテスト作成 | Step 1-5 |
| 7 | Makefile更新（`make test`） | Step 6 |
| 8 | MySQLデータダンプ & Supabase投入 | Step 5 |

---

## 7. 依存ライブラリ追加

```
gorm.io/driver/postgres  # PostgreSQL GORM driver
github.com/lib/pq        # PostgreSQL Go driver (pq)
```

---

## 8. リスク・注意事項

- `DATE_FORMAT` → `TO_CHAR` の変換漏れに注意（GetResults, GetOKexResults）
- PostgreSQLの`float`は`DOUBLE PRECISION`を使用（MySQLの`float`より精度が異なる可能性）
- `ON UPDATE CURRENT_TIMESTAMP` はPostgreSQLに存在しないためトリガーで代替
- OKEx系のテーブル名動的切り替え（`TableName`変数）はそのまま維持
- Supabaseの接続はSSL必須（DSNに`?sslmode=require`を付与）
