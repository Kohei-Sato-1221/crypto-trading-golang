package main

import (
	"log"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/app"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/config"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/models"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/okex"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/utils"
)

func main() {
	config.NewConfig()
	models.NewMysqlBase()
	utils.LogSetting(config.Config.LogFile)

	log.Printf("#######")
	log.Printf("#######")
	log.Printf("config:%#v", config.Config)
	log.Printf("#######")
	log.Printf("#######")

	useExchange := config.Config.Exchange

	if useExchange == "bitflyer" {
		okex.TableName = "buy_orders"
		app.StartBfService()
	}
	if useExchange == "okex" {
		okex.TableName = "buy_orders"
		okex.BaseURL = "https://www.okex.com"
		app.StartOKEXService(useExchange)
	}
	if useExchange == "okj" {
		okex.TableName = "okj_buy_orders"
		okex.BaseURL = "https://www.okcoin.jp"
		app.StartOKJService(useExchange)
	}
}
