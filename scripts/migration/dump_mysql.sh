#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
DUMP_DIR="$PROJECT_ROOT/dump"

mkdir -p "$DUMP_DIR"

echo "=== Dumping MySQL data from AWS RDS ==="
mysqldump -h crypto-trading-db.cva64ye44jkh.ap-northeast-1.rds.amazonaws.com \
  -P 1221 -u crypto_trading_root -p'CryptoSl0S&mDGdY098' \
  crypto_trading buy_orders sell_orders price_histories okj_buy_orders \
  --no-create-info --complete-insert --skip-lock-tables \
  > "$DUMP_DIR/mysql_data.sql"

echo "=== Dump complete: $DUMP_DIR/mysql_data.sql ==="
echo "=== Converting to PostgreSQL format ==="

# MySQL → PostgreSQL変換
cat "$DUMP_DIR/mysql_data.sql" \
  | sed 's/`//g' \
  | sed "s/\\\\'/\\'\\'/g" \
  | sed '/^\/\*!/d' \
  | sed '/^--/d' \
  | sed '/^LOCK TABLES/d' \
  | sed '/^UNLOCK TABLES/d' \
  | sed '/^SET /d' \
  | sed '/^$/d' \
  > "$DUMP_DIR/postgres_data.sql"

echo "=== Conversion complete: $DUMP_DIR/postgres_data.sql ==="
echo "=== Done! Run 'make migrate-import' to import to Supabase ==="
