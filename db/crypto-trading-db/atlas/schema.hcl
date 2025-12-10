schema "crypto_trading_db" {
  charset = "utf8mb4"
  collate = "utf8mb4_unicode_ci"
}

table "buy_orders" {
  schema = schema.crypto_trading_db
  comment = "bitflyer_buyorders"

  column "id" {
    type = int
    unsigned = true
    null = false
    auto_increment = true
  }

  column "order_id" {
    type = varchar(50)
    null = true
  }

  column "product_code" {
    type = varchar(50)
    null = true
  }

  column "side" {
    type = varchar(20)
    null = true
  }

  column "price" {
    type = float
    null = true
  }

  column "size" {
    type = float
    null = true
  }

  column "exchange" {
    type = varchar(50)
    null = true
  }

  column "status" {
    type = varchar(100)
    null = true
    default = "UNFILLED"
    comment = "UNFILLED / FILLED / FILLED(SELL ORDER PLACED) / CANCELLED"
  }

  column "strategy" {
    type = tinyint
    null = false
    default = 99
    comment = "99:not recorded"
  }

  column "remarks" {
    type = text
    null = true
  }

  column "timestamp" {
    type = timestamp
    null = false
    default = sql("CURRENT_TIMESTAMP")
  }

  column "updatetime" {
    type = timestamp
    null = false
    default = sql("CURRENT_TIMESTAMP")
    on_update = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "orderId" {
    unique = true
    columns = [column.order_id]
  }
}

table "sell_orders" {
  schema = schema.crypto_trading_db
  comment = "bitflyer_sellorders"

  column "id" {
    type = int
    unsigned = true
    null = false
    auto_increment = true
  }

  column "parentid" {
    type = varchar(50)
    null = true
  }

  column "order_id" {
    type = varchar(50)
    null = true
  }

  column "product_code" {
    type = varchar(50)
    null = true
  }

  column "side" {
    type = varchar(20)
    null = true
  }

  column "price" {
    type = float
    null = true
  }

  column "size" {
    type = float
    null = true
  }

  column "exchange" {
    type = varchar(50)
    null = true
  }

  column "status" {
    type = varchar(100)
    null = true
    default = "UNFILLED"
    comment = "UNFILLED / FILLED / CANCELLED"
  }

  column "remarks" {
    type = text
    null = true
  }

  column "timestamp" {
    type = timestamp
    null = false
    default = sql("CURRENT_TIMESTAMP")
  }

  column "updatetime" {
    type = timestamp
    null = false
    default = sql("CURRENT_TIMESTAMP")
    on_update = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "orderId" {
    unique = true
    columns = [column.order_id]
  }
}

table "price_histories" {
  schema = schema.crypto_trading_db
  comment = "価格履歴テーブル"

  column "id" {
    type = int
    unsigned = true
    null = false
    auto_increment = true
  }

  column "datetime" {
    type = timestamp
    null = false
    default = sql("CURRENT_TIMESTAMP")
  }

  column "product_code" {
    type = varchar(50)
    null = false
  }

  column "price" {
    type = float
    null = false
  }

  column "price_ratio_24h" {
    type = float
    null = true
    comment = "24時間前との価格比率（少数形式: 例: 0.95 = 95%, 1.21 = 121%）"
  }

  column "created_at" {
    type = timestamp
    null = false
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_product_code_datetime" {
    columns = [column.product_code, column.datetime]
  }
}

