#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
DUMP_DIR="$PROJECT_ROOT/dump"

SUPABASE_DSN="postgresql://postgres.hastyhkkwedwqchtlsiv:AbcfbG294QB2hR@aws-1-ap-northeast-1.pooler.supabase.com:6543/postgres?sslmode=require"

if [ ! -f "$DUMP_DIR/postgres_data.sql" ]; then
  echo "ERROR: $DUMP_DIR/postgres_data.sql not found. Run 'make migrate-dump' first."
  exit 1
fi

echo "=== Creating tables (if not exist) ==="
psql "$SUPABASE_DSN" -f "$PROJECT_ROOT/db/crypto-trading-db-postgres/init/001_init.sql"

echo "=== Importing data to Supabase ==="
psql "$SUPABASE_DSN" -f "$DUMP_DIR/postgres_data.sql"

echo "=== Resetting sequences ==="
psql "$SUPABASE_DSN" -c "
SELECT setval('buy_orders_id_seq', COALESCE((SELECT MAX(id) FROM buy_orders), 0) + 1, false);
SELECT setval('sell_orders_id_seq', COALESCE((SELECT MAX(id) FROM sell_orders), 0) + 1, false);
SELECT setval('price_histories_id_seq', COALESCE((SELECT MAX(id) FROM price_histories), 0) + 1, false);
SELECT setval('okj_buy_orders_id_seq', COALESCE((SELECT MAX(id) FROM okj_buy_orders), 0) + 1, false);
"

echo "=== Import complete! ==="
