package main

import (
	"config"
	"controller"
	"fmt"
	"models"
	"utils"
)

func main() {
	//useExchange = "bitflyer"
	// useExchange := "okex"
	useExchange := "okj"

	utils.LogSetting(config.Config.LogFile)
	fmt.Println(models.MysqlDbConn)

	//if useExchange == "bitflyer" {
	//	controller.StartBfService()
	//}
	// if useExchange == "okex" {
	// 	controller.StartOKEXService(useExchange)
	// }
	if useExchange == "okj" {
		controller.StartOKJService(useExchange)
	}
}
