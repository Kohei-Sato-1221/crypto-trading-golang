package tests

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/database"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

const testDSN = "postgresql://postgres:postgres@localhost:5433/crypto_trading?sslmode=disable"

func setupTestDB(t *testing.T) {
	t.Helper()

	// 環境変数でDSN上書き可能
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = testDSN
	}

	client := database.NewPostgresClient(dsn)
	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	database.Current = client
	models.AppDB = client.GetDB()
	models.GormDB = client.GetGormDB()

	// テーブルをクリーンアップ
	cleanupTables(t)
}

func teardownTestDB(t *testing.T) {
	t.Helper()
	cleanupTables(t)
	if database.Current != nil {
		database.Current.Close()
	}
}

func cleanupTables(t *testing.T) {
	t.Helper()
	tables := []string{"sell_orders", "buy_orders", "price_histories", "okj_buy_orders"}
	for _, table := range tables {
		_, err := models.AppDB.Exec(fmt.Sprintf("DELETE FROM %s", table))
		if err != nil {
			t.Logf("Warning: failed to clean table %s: %v", table, err)
		}
	}
	// シーケンスリセット
	sequences := []string{"buy_orders_id_seq", "sell_orders_id_seq", "price_histories_id_seq", "okj_buy_orders_id_seq"}
	for _, seq := range sequences {
		models.AppDB.Exec(fmt.Sprintf("ALTER SEQUENCE %s RESTART WITH 1", seq))
	}
}

func insertTestBuyOrder(t *testing.T, orderID string, productCode string, price float64, size float64, status string) {
	t.Helper()
	_, err := models.AppDB.Exec(
		`INSERT INTO buy_orders (order_id, product_code, side, price, size, exchange, status, strategy, remarks)
		 VALUES ($1, $2, 'BUY', $3, $4, 'bitflyer', $5, 1, 'test')`,
		orderID, productCode, price, size, status,
	)
	if err != nil {
		t.Fatalf("Failed to insert test buy order: %v", err)
	}
}

func insertTestSellOrder(t *testing.T, parentID, orderID, productCode string, price float64, size float64, status string) {
	t.Helper()
	_, err := models.AppDB.Exec(
		`INSERT INTO sell_orders (parentid, order_id, product_code, side, price, size, exchange, status)
		 VALUES ($1, $2, $3, 'SELL', $4, $5, 'bitflyer', $6)`,
		parentID, orderID, productCode, price, size, status,
	)
	if err != nil {
		t.Fatalf("Failed to insert test sell order: %v", err)
	}
}

func insertTestPriceHistory(t *testing.T, productCode string, price float64, datetime time.Time, priceRatio24h *float64) {
	t.Helper()
	_, err := models.AppDB.Exec(
		`INSERT INTO price_histories (datetime, product_code, price, price_ratio_24h)
		 VALUES ($1, $2, $3, $4)`,
		datetime, productCode, price, priceRatio24h,
	)
	if err != nil {
		t.Fatalf("Failed to insert test price history: %v", err)
	}
}

func getDB(t *testing.T) *sql.DB {
	t.Helper()
	if models.AppDB == nil {
		t.Fatal("AppDB is nil - call setupTestDB first")
	}
	return models.AppDB
}

func init() {
	log.SetOutput(os.Stdout)
}
