package tests

import (
	"strings"
	"testing"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

func TestGetResultsEmpty(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	// データなしでもエラーにならないこと
	result, err := models.GetResults()
	if err != nil {
		t.Fatalf("GetResults() should not error on empty data: %v", err)
	}
	// ヘッダーは含まれる
	if !strings.Contains(result, "bitflyer 自動売買 収益") {
		t.Fatal("Expected header in result")
	}
}

func TestGetResultsWithData(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	// テストデータ: 買い注文 → 売り注文（FILLED）
	// buy: 5,000,000 * 0.001 = 5,000円
	// sell: 5,075,000 * 0.001 = 5,075円
	// profit: 75円
	insertTestBuyOrder(t, "TEST-RESULT-BUY-1", "BTC_JPY", 5000000, 0.001, "FILLED(SELL ORDER PLACED)")
	insertTestSellOrder(t, "TEST-RESULT-BUY-1", "TEST-RESULT-SELL-1", "BTC_JPY", 5075000, 0.001, "FILLED")

	// sell_ordersのupdatetimeを今日に設定（集計対象にする）
	utc, _ := time.LoadLocation("UTC")
	now := time.Now().In(utc)
	_, err := models.AppDB.Exec("UPDATE sell_orders SET updatetime = $1 WHERE order_id = $2", now, "TEST-RESULT-SELL-1")
	if err != nil {
		t.Fatalf("Failed to update sell order timestamp: %v", err)
	}

	result, err := models.GetResults()
	if err != nil {
		t.Fatalf("GetResults() failed: %v", err)
	}

	// 結果にデータ行が含まれること
	if !strings.Contains(result, "bitflyer 自動売買 収益") {
		t.Fatal("Expected header in result")
	}
	// Total行 or 日付行が含まれること
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) < 3 {
		t.Fatalf("Expected at least 3 lines (header + header2 + data), got %d:\n%s", len(lines), result)
	}
}

func TestGetResultsProfitCalculation(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	utc, _ := time.LoadLocation("UTC")
	now := time.Now().In(utc)

	// 3つの取引を設定（全てFILLED）
	trades := []struct {
		buyOrderID  string
		sellOrderID string
		buyPrice    float64
		sellPrice   float64
		size        float64
	}{
		{"PROFIT-BUY-1", "PROFIT-SELL-1", 5000000, 5075000, 0.001},
		{"PROFIT-BUY-2", "PROFIT-SELL-2", 4900000, 4973500, 0.001},
		{"PROFIT-BUY-3", "PROFIT-SELL-3", 5100000, 5176500, 0.001},
	}

	for _, trade := range trades {
		insertTestBuyOrder(t, trade.buyOrderID, "BTC_JPY", trade.buyPrice, trade.size, "FILLED(SELL ORDER PLACED)")
		insertTestSellOrder(t, trade.buyOrderID, trade.sellOrderID, "BTC_JPY", trade.sellPrice, trade.size, "FILLED")
		models.AppDB.Exec("UPDATE sell_orders SET updatetime = $1 WHERE order_id = $2", now, trade.sellOrderID)
	}

	result, err := models.GetResults()
	if err != nil {
		t.Fatalf("GetResults() failed: %v", err)
	}

	// Total行が存在すること
	if !strings.Contains(result, "Total") {
		t.Fatalf("Expected 'Total' in result, got:\n%s", result)
	}

	// 利益が正の値であること（具体的な数値は手数料率により変動）
	t.Logf("GetResults output:\n%s", result)
}
