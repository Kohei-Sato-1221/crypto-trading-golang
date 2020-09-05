package main

import (
	"config"
	"controller"
	"fmt"
	"models"
	"okex"
	"utils"
)

func main() {
	//useExchange = "bitflyer"
	// useExchange := "okex"
	useExchange := "okj"

	utils.LogSetting(config.Config.LogFile)
	fmt.Println(models.MysqlDbConn)

	//if useExchange == "bitflyer" {
	//	models.TableName = "buy_orders"
	//	controller.StartBfService()
	//}
	// if useExchange == "okex" {
	//	models.TableName = "buy_orders"
	//  okex.BaseURL = "https://www.okex.com"
	// 	controller.StartOKEXService(useExchange)
	// }
	if useExchange == "okj" {
		models.TableName = "okj_buy_orders"
		okex.BaseURL = "https://www.okcoin.jp"
		controller.StartOKJService(useExchange)
	}
}
