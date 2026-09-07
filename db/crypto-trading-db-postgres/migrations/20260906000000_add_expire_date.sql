-- 注文の有効期限(expire_date)を記録できるようにする。
-- 失効検出(expireSweepJob)と売り注文の27日ローリング(rolloverSellOrderJob)の基盤。
-- 既存行には正しい値が存在しない(取引所APIは失効注文の期限を返さない)ため NULL 許容。

ALTER TABLE buy_orders  ADD COLUMN IF NOT EXISTS expire_date TIMESTAMP NULL;
ALTER TABLE sell_orders ADD COLUMN IF NOT EXISTS expire_date TIMESTAMP NULL;

COMMENT ON COLUMN buy_orders.expire_date  IS '注文の有効期限(UTC)。取引所APIのexpire_dateまたは発注時刻+minute_to_expire。NULLは期限不明';
COMMENT ON COLUMN sell_orders.expire_date IS '注文の有効期限(UTC)。取引所APIのexpire_dateまたは発注時刻+minute_to_expire。NULLは期限不明';

-- sweep / ローリングの抽出クエリ用
CREATE INDEX IF NOT EXISTS idx_buy_orders_status_expire  ON buy_orders  (status, expire_date);
CREATE INDEX IF NOT EXISTS idx_sell_orders_status_expire ON sell_orders (status, expire_date);

-- strategy の拡幅(任意): PostgreSQL では smallint(上限32767)で戦略値10001〜20003は既に格納可能。
-- 将来の余裕のための変更であり、適用しなくても機能は正しく動作する。
ALTER TABLE buy_orders ALTER COLUMN strategy TYPE INTEGER;
COMMENT ON COLUMN buy_orders.strategy IS '買い戦略ID。99=未記録, 127=MySQL時代のtinyint飽和値(集計から除外)';
