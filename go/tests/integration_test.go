package tests

import (
	"fmt"
	"testing"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/database"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

func TestDBConnection(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	if models.AppDB == nil {
		t.Fatal("AppDB should not be nil after setup")
	}
	if models.GormDB == nil {
		t.Fatal("GormDB should not be nil after setup")
	}
	if database.CurrentDriver() != "postgres" {
		t.Fatalf("Expected driver 'postgres', got '%s'", database.CurrentDriver())
	}

	err := models.AppDB.Ping()
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestInsertBuyOrder(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	event := &models.OrderEvent{
		OrderID:     "TEST-BUY-001",
		ProductCode: "BTC_JPY",
		Side:        "BUY",
		Price:       5000000,
		Size:        0.001,
		Exchange:    "bitflyer",
		Strategy:    1,
	}

	err := event.BuyOrder()
	if err != nil {
		t.Fatalf("BuyOrder() failed: %v", err)
	}

	// 挿入確認
	var count int
	err = models.AppDB.QueryRow("SELECT COUNT(*) FROM buy_orders WHERE order_id = $1", "TEST-BUY-001").Scan(&count)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("Expected 1 buy order, got %d", count)
	}
}

func TestInsertBuyOrderDuplicate(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	event := &models.OrderEvent{
		OrderID:     "TEST-BUY-DUP",
		ProductCode: "BTC_JPY",
		Side:        "BUY",
		Price:       5000000,
		Size:        0.001,
		Exchange:    "bitflyer",
		Strategy:    1,
	}

	err := event.BuyOrder()
	if err != nil {
		t.Fatalf("First BuyOrder() failed: %v", err)
	}

	// 重複INSERTはエラーにならない（nilが返る）
	err = event.BuyOrder()
	if err != nil {
		t.Fatalf("Duplicate BuyOrder() should not error, got: %v", err)
	}
}

func TestInsertSellOrder(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	// 先に親の買い注文を作成
	insertTestBuyOrder(t, "TEST-BUY-PARENT", "BTC_JPY", 5000000, 0.001, "FILLED")

	event := &models.OrderEvent{
		OrderID:     "TEST-SELL-001",
		ProductCode: "BTC_JPY",
		Side:        "SELL",
		Price:       5075000,
		Size:        0.001,
		Exchange:    "bitflyer",
	}

	err := event.SellOrder("TEST-BUY-PARENT")
	if err != nil {
		t.Fatalf("SellOrder() failed: %v", err)
	}

	var count int
	err = models.AppDB.QueryRow("SELECT COUNT(*) FROM sell_orders WHERE order_id = $1", "TEST-SELL-001").Scan(&count)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("Expected 1 sell order, got %d", count)
	}
}

func TestGetUnfilledBuyOrders(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	insertTestBuyOrder(t, "TEST-UNFILLED-1", "BTC_JPY", 5000000, 0.001, "UNFILLED")
	insertTestBuyOrder(t, "TEST-UNFILLED-2", "ETH_JPY", 300000, 0.01, "UNFILLED")
	insertTestBuyOrder(t, "TEST-FILLED-1", "BTC_JPY", 4900000, 0.001, "FILLED")

	orders, err := models.GetUnfilledBuyOrders()
	if err != nil {
		t.Fatalf("GetUnfilledBuyOrders() failed: %v", err)
	}
	if len(orders) != 2 {
		t.Fatalf("Expected 2 unfilled orders, got %d", len(orders))
	}
}

func TestUpdateFilledOrder(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	insertTestBuyOrder(t, "TEST-UPDATE-BUY", "BTC_JPY", 5000000, 0.001, "UNFILLED")
	insertTestSellOrder(t, "TEST-UPDATE-BUY", "TEST-UPDATE-SELL", "BTC_JPY", 5075000, 0.001, "UNFILLED")

	err := models.UpdateFilledOrder("TEST-UPDATE-BUY")
	if err != nil {
		t.Fatalf("UpdateFilledOrder() failed: %v", err)
	}

	// buy_orderのステータス確認
	var status string
	err = models.AppDB.QueryRow("SELECT status FROM buy_orders WHERE order_id = $1", "TEST-UPDATE-BUY").Scan(&status)
	if err != nil {
		t.Fatalf("Query buy_orders failed: %v", err)
	}
	if status != "FILLED" {
		t.Fatalf("Expected status 'FILLED', got '%s'", status)
	}
}

func TestShouldPlaceBuyOrder(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	// 注文なし → 発注可能
	shouldSkip, err, _ := models.ShouldPlaceBuyOrder(5, 5)
	if err != nil {
		t.Fatalf("ShouldPlaceBuyOrder() failed: %v", err)
	}
	if shouldSkip {
		t.Fatal("Expected shouldSkip=false when no orders exist")
	}

	// 未約定の買い注文を5つ追加
	for i := 0; i < 5; i++ {
		insertTestBuyOrder(t, fmt.Sprintf("TEST-MAX-%d", i), "BTC_JPY", 5000000, 0.001, "UNFILLED")
	}

	// max_buy_orders=5に達している → 発注不可
	shouldSkip, err, _ = models.ShouldPlaceBuyOrder(5, 10)
	if err != nil {
		t.Fatalf("ShouldPlaceBuyOrder() failed: %v", err)
	}
	if !shouldSkip {
		t.Fatal("Expected shouldSkip=true when max buy orders reached")
	}
}
