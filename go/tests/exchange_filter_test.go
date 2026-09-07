package tests

import (
	"testing"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

/*
F11 の回帰テスト。

buy_orders / sell_orders は OKEX サービスと共用のテーブル（cmds のエントリポイントで
okex.TableName = "buy_orders" が設定される）。Bitflyer 用のジョブが取引所を絞らずに
UNFILLED を拾うと、OKEX 由来のレコードに対して Bitflyer の CancelOrder / 一覧取得を
OKEX の product_code で叩くことになり、毎日のSlackエラーが恒常化する。

Bitflyer 専用のSELECTである GetUnfilledBuyOrderRecords / GetSellOrdersToRollover が
exchange = 'bitflyer' で絞れていること、および Bitflyer のレコードは
従来どおり全件返ること（リグレッション）を確認する。
*/

func insertBuyOrderWithExchange(t *testing.T, orderID, productCode, exchange, status string, timestamp time.Time) {
	t.Helper()
	_, err := models.AppDB.Exec(
		`INSERT INTO buy_orders (order_id, product_code, side, price, size, exchange, status, strategy, timestamp)
		 VALUES ($1, $2, 'BUY', 5000000, 0.001, $3, $4, 10001, $5)`,
		orderID, productCode, exchange, status, timestamp)
	if err != nil {
		t.Fatalf("insert buy_orders failed: %v", err)
	}
}

func insertSellOrderWithExchange(t *testing.T, parentID, orderID, productCode, exchange string,
	expireDate time.Time, timestamp time.Time) {
	t.Helper()
	_, err := models.AppDB.Exec(
		`INSERT INTO sell_orders (parentid, order_id, product_code, side, price, size, exchange, status, expire_date, timestamp)
		 VALUES ($1, $2, $3, 'SELL', 5100000, 0.001, $4, 'UNFILLED', $5, $6)`,
		parentID, orderID, productCode, exchange, expireDate, timestamp)
	if err != nil {
		t.Fatalf("insert sell_orders failed: %v", err)
	}
}

// GetUnfilledBuyOrderRecords が OKEX 由来のレコードを拾わないこと。
func TestGetUnfilledBuyOrderRecordsFiltersByExchange(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	now := time.Now().UTC()
	insertBuyOrderWithExchange(t, "B-BF-1", "BTC_JPY", models.ExchangeBitflyer, "UNFILLED", now.AddDate(0, 0, -10))
	insertBuyOrderWithExchange(t, "B-BF-2", "ETH_JPY", models.ExchangeBitflyer, "UNFILLED", now.AddDate(0, 0, -5))
	insertBuyOrderWithExchange(t, "B-OKEX", "BTC-USDT", "okex", "UNFILLED", now.AddDate(0, 0, -3))
	// リグレッション: exchange に関わらず UNFILLED 以外は従来どおり対象外
	insertBuyOrderWithExchange(t, "B-BF-FILLED", "BTC_JPY", models.ExchangeBitflyer, "FILLED", now.AddDate(0, 0, -1))

	records, err := models.GetUnfilledBuyOrderRecords(10)
	if err != nil {
		t.Fatalf("GetUnfilledBuyOrderRecords failed: %v", err)
	}

	got := map[string]bool{}
	for _, record := range records {
		got[record.OrderID] = true
		if record.Exchange != models.ExchangeBitflyer {
			t.Errorf("bitflyer 以外のレコードが混ざっている: %+v", record)
		}
	}
	if got["B-OKEX"] {
		t.Error("OKEX 由来のレコードを拾っている（Bitflyer の CancelOrder を誤った product_code で叩く）")
	}
	if !got["B-BF-1"] || !got["B-BF-2"] {
		t.Errorf("Bitflyer の未約定レコードが取りこぼされている: %v", got)
	}
	if got["B-BF-FILLED"] {
		t.Error("FILLED が対象に含まれている")
	}
	if len(records) != 2 {
		t.Errorf("records = %d, want 2 (%v)", len(records), got)
	}
}

// GetSellOrdersToRollover が OKEX 由来のレコードを拾わないこと。
func TestGetSellOrdersToRolloverFiltersByExchange(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	now := time.Now().UTC()
	expireSoon := now.AddDate(0, 0, 2) // 期限まで2日 → daysBefore=3 の対象
	insertSellOrderWithExchange(t, "P-BF", "S-BF", "BTC_JPY", models.ExchangeBitflyer, expireSoon, now.AddDate(0, 0, -28))
	insertSellOrderWithExchange(t, "P-OKEX", "S-OKEX", "BTC-USDT", "okex", expireSoon, now.AddDate(0, 0, -28))

	records, err := models.GetSellOrdersToRollover(now, 3, 27, 20)
	if err != nil {
		t.Fatalf("GetSellOrdersToRollover failed: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("records = %d, want 1 (%+v)", len(records), records)
	}
	if records[0].OrderID != "S-BF" {
		t.Errorf("OrderID = %s, want S-BF（OKEX 由来のレコードを拾っている）", records[0].OrderID)
	}
	if records[0].Exchange != models.ExchangeBitflyer {
		t.Errorf("Exchange = %s, want %s", records[0].Exchange, models.ExchangeBitflyer)
	}
}
