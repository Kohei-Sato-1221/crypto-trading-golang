package main

import (
	"log"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/app"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/config"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/okex"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/utils"
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
