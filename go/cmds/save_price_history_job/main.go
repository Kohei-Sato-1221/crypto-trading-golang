package main

import (
	"log"

	app "github.com/Kohei-Sato-1221/crypto-trading-golang/go/app/bitflyerApp"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/bitflyer"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/config"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/utils"
)

func main() {
	log.Println("🔷Start savePriceHistoryJob🔷")
	config.NewConfig()
	models.InitDB()
	utils.LogSetting(config.Config.LogFile)
	log.Printf("#######\n")
	log.Printf("config:%#v\n", config.Config)
	log.Printf("#######\n")
	log.Println()

	// APIクライアントの初期化
	apiClient := bitflyer.NewBitflyer(
		config.Config.ApiKey,
		config.Config.ApiSecret,
		config.Config.BFMaxBuy,
		config.Config.BFMaxSell,
	)

	// Slackクライアントの初期化
	app.InitSlackClient()

	// ジョブの実行
	app.SavePriceHistoryJob(apiClient)

	log.Println("🔷End savePriceHistoryJob🔷")
}
