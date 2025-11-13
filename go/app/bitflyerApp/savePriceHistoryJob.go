package bitflyerApp

import (
	"log"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/bitflyer"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

func savePriceHistoryJob(apiClient *bitflyer.APIClient) {
	log.Println("【savePriceHistoryJob】Start of job")

	productCodes := []string{"BTC_JPY", "ETH_JPY", "XRP_JPY", "MONA_JPY"}
	for _, productCode := range productCodes {
		savePriceHistoryForProduct(apiClient, productCode)
	}

	log.Println("【savePriceHistoryJob】End of job")
}

func savePriceHistoryForProduct(apiClient *bitflyer.APIClient, productCode string) {
	ticker, err := apiClient.GetTicker(productCode)
	if err != nil {
		log.Printf("【ERROR】Failed to get %s ticker: %v", productCode, err)
		slackClient.PostMessage("【ERROR】Failed to get "+productCode+" ticker", true)
		return
	}

	// 24時間前の価格を取得
	price24hAgo, err := models.GetPrice24HoursAgo(productCode)
	if err != nil {
		log.Printf("【ERROR】Failed to get %s price 24h ago: %v", productCode, err)
	}

	var priceRatio24h *float64
	if price24hAgo != nil && *price24hAgo > 0 {
		ratio := ticker.Ltp / *price24hAgo
		priceRatio24h = &ratio
		log.Printf("%s: current=%.2f, 24h_ago=%.2f, ratio=%.4f", productCode, ticker.Ltp, *price24hAgo, ratio)
	} else {
		log.Printf("%s: current=%.2f, 24h_ago=no data", productCode, ticker.Ltp)
	}

	err = models.SavePriceHistory(productCode, ticker.Ltp, priceRatio24h)
	if err != nil {
		log.Printf("【ERROR】Failed to save %s price history: %v", productCode, err)
		slackClient.PostMessage("【ERROR】Failed to save "+productCode+" price history", true)
	}
}
