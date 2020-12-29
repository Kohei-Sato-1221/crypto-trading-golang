package main

import (
	"fmt"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/config"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/app"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/models"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/okex"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/utils"
)

func main() {
	utils.LogSetting(config.Config.LogFile)
	fmt.Println(models.MysqlDbConn)

	useExchange := config.Config.Exchange

	if useExchange == "bitflyer" {
		models.TableName = "buy_orders"
		app.StartBfService()
	}
	if useExchange == "okex" {
		models.TableName = "buy_orders"
	 okex.BaseURL = "https://www.okex.com"
		app.StartOKEXService(useExchange)
	}
	if useExchange == "okj" {
		models.TableName = "okj_buy_orders"
		okex.BaseURL = "https://www.okcoin.jp"
		app.StartOKJService(useExchange)
	}
}
