package main

import (
	"config"
	"controller"
	"fmt"
	"models"
	"utils"
)

func main() {
	// useExchange := config.Config.Exchange
	useExchange := "bitflyer"
	utils.LogSetting(config.Config.LogFile)
	fmt.Println(models.MysqlDbConn)

	if useExchange == "bitflyer" {
		controller.StartBfService()
	}
	// if useExchange == "okex" {
	// 	fmt.Println(models.MysqlDbConn)
	// 	controller.StartOKEXService()
	// }
}
