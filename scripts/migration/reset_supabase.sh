#!/bin/bash
set -e

SUPABASE_DSN="postgresql://postgres:AbcfbG294QB2hR@db.hastyhkkwedwqchtlsiv.supabase.co:5432/postgres?sslmode=require"

echo "=== WARNING: This will DELETE ALL DATA in Supabase ==="
read -p "Are you sure? (yes/no): " confirm
if [ "$confirm" != "yes" ]; then
  echo "Cancelled."
  exit 0
fi

echo "=== Dropping all tables and triggers ==="
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
