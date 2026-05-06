-- PostgreSQL schema for crypto_trading
-- This file is used by docker-compose for local development initialization

-- buy_orders (bitflyer)
CREATE TABLE IF NOT EXISTS buy_orders (
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

-- sell_orders (bitflyer)
CREATE TABLE IF NOT EXISTS sell_orders (
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
CREATE TABLE IF NOT EXISTS price_histories (
    id SERIAL PRIMARY KEY,
    datetime TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    product_code VARCHAR(50) NOT NULL,
    price DOUBLE PRECISION NOT NULL,
    price_ratio_24h DOUBLE PRECISION,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_product_code_datetime ON price_histories (product_code, datetime);

-- okj_buy_orders (OKJ)
CREATE TABLE IF NOT EXISTS okj_buy_orders (
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

-- updatetime自動更新トリガー（PostgreSQLにはON UPDATE CURRENT_TIMESTAMPがないため）
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
