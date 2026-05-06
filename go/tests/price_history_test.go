package tests

import (
	"math"
	"testing"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

func TestSavePriceHistory(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	err := models.SavePriceHistory("BTC_JPY", 5000000, nil)
	if err != nil {
		t.Fatalf("SavePriceHistory() failed: %v", err)
	}

	// 挿入確認
	var count int
	err = models.AppDB.QueryRow("SELECT COUNT(*) FROM price_histories WHERE product_code = $1", "BTC_JPY").Scan(&count)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("Expected 1 price history, got %d", count)
	}
}

func TestSavePriceHistoryWithRatio(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	ratio := 0.95
	err := models.SavePriceHistory("ETH_JPY", 300000, &ratio)
	if err != nil {
		t.Fatalf("SavePriceHistory() with ratio failed: %v", err)
	}

	var savedRatio float64
	err = models.AppDB.QueryRow("SELECT price_ratio_24h FROM price_histories WHERE product_code = $1", "ETH_JPY").Scan(&savedRatio)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if math.Abs(savedRatio-0.95) > 0.001 {
		t.Fatalf("Expected ratio 0.95, got %f", savedRatio)
	}
}

func TestGetPrice24HoursAgo(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	utc, _ := time.LoadLocation("UTC")
	now := time.Now().In(utc)

	// 24時間前のデータを挿入
	insertTestPriceHistory(t, "BTC_JPY", 4800000, now.Add(-24*time.Hour), nil)
	// 12時間前のデータを挿入
	insertTestPriceHistory(t, "BTC_JPY", 4900000, now.Add(-12*time.Hour), nil)
	// 現在のデータ
	insertTestPriceHistory(t, "BTC_JPY", 5000000, now, nil)

	price, err := models.GetPrice24HoursAgo("BTC_JPY")
	if err != nil {
		t.Fatalf("GetPrice24HoursAgo() failed: %v", err)
	}
	if price == nil {
		t.Fatal("Expected price, got nil")
	}
	// 23時間前以前の最新レコード = 24時間前のレコード (4800000)
	if math.Abs(*price-4800000) > 1 {
		t.Fatalf("Expected price ~4800000, got %f", *price)
	}
}

func TestGetPrice24HoursAgoNoData(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	// データなしの場合はnil（エラーではない）
	price, err := models.GetPrice24HoursAgo("BTC_JPY")
	if err != nil {
		t.Fatalf("GetPrice24HoursAgo() should not error on no data: %v", err)
	}
	if price != nil {
		t.Fatalf("Expected nil price when no data, got %f", *price)
	}
}

func TestGetLowestPriceInPast7Days(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	utc, _ := time.LoadLocation("UTC")
	now := time.Now().In(utc)

	// 過去7日以内のデータ
	insertTestPriceHistory(t, "BTC_JPY", 5200000, now.Add(-1*24*time.Hour), nil)
	insertTestPriceHistory(t, "BTC_JPY", 4700000, now.Add(-3*24*time.Hour), nil) // 最低
	insertTestPriceHistory(t, "BTC_JPY", 5100000, now.Add(-5*24*time.Hour), nil)
	// 8日前（範囲外）
	insertTestPriceHistory(t, "BTC_JPY", 4000000, now.Add(-8*24*time.Hour), nil)

	price, err := models.GetLowestPriceInPast7Days("BTC_JPY")
	if err != nil {
		t.Fatalf("GetLowestPriceInPast7Days() failed: %v", err)
	}
	if price == nil {
		t.Fatal("Expected price, got nil")
	}
	if math.Abs(*price-4700000) > 1 {
		t.Fatalf("Expected lowest price ~4700000, got %f", *price)
	}
}

func TestSavePriceHistoryFlow(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	utc, _ := time.LoadLocation("UTC")
	now := time.Now().In(utc)

	// Step 1: 24時間前の価格を保存
	insertTestPriceHistory(t, "BTC_JPY", 4800000, now.Add(-24*time.Hour), nil)

	// Step 2: 現在の価格を保存（ratioなし）
	err := models.SavePriceHistory("BTC_JPY", 5000000, nil)
	if err != nil {
		t.Fatalf("SavePriceHistory() failed: %v", err)
	}

	// Step 3: 24時間前の価格を取得
	price24hAgo, err := models.GetPrice24HoursAgo("BTC_JPY")
	if err != nil {
		t.Fatalf("GetPrice24HoursAgo() failed: %v", err)
	}
	if price24hAgo == nil {
		t.Fatal("Expected price 24h ago, got nil")
	}

	// Step 4: ratio計算して再保存
	ratio := 5000000 / *price24hAgo
	err = models.SavePriceHistory("BTC_JPY", 5100000, &ratio)
	if err != nil {
		t.Fatalf("SavePriceHistory() with ratio failed: %v", err)
	}

	// 検証: 3レコード存在する
	var count int
	models.AppDB.QueryRow("SELECT COUNT(*) FROM price_histories WHERE product_code = $1", "BTC_JPY").Scan(&count)
	if count != 3 {
		t.Fatalf("Expected 3 price history records, got %d", count)
	}
}
