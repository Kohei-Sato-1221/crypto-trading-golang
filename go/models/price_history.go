package models

import (
	"log"
	"time"
)

type PriceHistory struct {
	ID            uint      `gorm:"primary_key"`
	DateTime      time.Time `gorm:"column:datetime"`
	ProductCode   string    `gorm:"column:product_code"`
	Price         float64   `gorm:"column:price"`
	PriceRatio24h *float64  `gorm:"column:price_ratio_24h"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

// SavePriceHistory 価格履歴を保存する
func SavePriceHistory(productCode string, price float64, priceRatio24h *float64) error {
	utc, _ := time.LoadLocation("UTC")
	now := time.Now().In(utc)

	cmd, err := AppDB.Prepare("INSERT INTO price_histories (datetime, product_code, price, price_ratio_24h) VALUES (?, ?, ?, ?)")
	if err != nil {
		log.Printf("[ERROR] SavePriceHistory01: %s\n", err)
		return err
	}

	_, err = cmd.Exec(now, productCode, price, priceRatio24h)
	if err != nil {
		log.Printf("[ERROR] SavePriceHistory02: %s\n", err)
		return err
	}

	log.Printf("Saved price history: product_code=%s, price=%.2f, ratio_24h=%v", productCode, price, priceRatio24h)
	return nil
}

// GetPrice24HoursAgo 24時間前の価格を取得する
// 実行タイミングから23時間前を基準として、その時点以前の最新レコードを取得する
func GetPrice24HoursAgo(productCode string) (*float64, error) {
	utc, _ := time.LoadLocation("UTC")
	now := time.Now().In(utc)
	// 23時間前を基準として、その時点以前の最新レコードを取得
	twentyThreeHoursAgo := now.Add(-23 * time.Hour)

	cmd, err := AppDB.Prepare("SELECT price FROM price_histories WHERE product_code = ? AND datetime <= ? ORDER BY datetime DESC LIMIT 1")
	if err != nil {
		log.Printf("[ERROR] GetPrice24HoursAgo01: %s\n", err)
		return nil, err
	}

	var price *float64
	err = cmd.QueryRow(productCode, twentyThreeHoursAgo).Scan(&price)
	if err != nil {
		// 23時間前以前のデータがない場合はnilを返す（エラーではない）
		log.Printf("No price data found before 23 hours ago for %s (searched before %v)", productCode, twentyThreeHoursAgo)
		return nil, nil
	}

	log.Printf("Found price 24h ago for %s: price=%.2f (searched before %v)", productCode, *price, twentyThreeHoursAgo)
	return price, nil
}

// GetLowestPriceInPast7Days 過去7日間の最低価格を取得する
func GetLowestPriceInPast7Days(productCode string) (*float64, error) {
	utc, _ := time.LoadLocation("UTC")
	now := time.Now().In(utc)
	sevenDaysAgo := now.Add(-7 * 24 * time.Hour)

	cmd, err := AppDB.Prepare("SELECT MIN(price) FROM price_histories WHERE product_code = ? AND datetime >= ?")
	if err != nil {
		log.Printf("[ERROR] GetLowestPriceInPast7Days01: %s\n", err)
		return nil, err
	}

	var price *float64
	err = cmd.QueryRow(productCode, sevenDaysAgo).Scan(&price)
	if err != nil {
		log.Printf("No price data found in past 7 days for %s (searched from %v)", productCode, sevenDaysAgo)
		return nil, nil
	}

	if price != nil {
		log.Printf("Found lowest price in past 7 days for %s: price=%.2f (searched from %v)", productCode, *price, sevenDaysAgo)
	}
	return price, nil
}
