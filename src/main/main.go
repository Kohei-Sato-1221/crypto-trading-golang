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
	useExchange := "okex"
	utils.LogSetting(config.Config.LogFile)

	if useExchange == "bitflyer" {
		fmt.Println(models.DbConnection)
		controller.StartBfService()
	}
	if useExchange == "okex" {
		fmt.Println(models.MysqlDbConn)
		controller.StartOKEXService()
	}
}
