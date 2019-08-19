package main

import (
//	"log"
	"config"
	"utils"
	"fmt"
	"models"
	"controller"
)

func main(){
	useExchange := config.Config.Exchange
	utils.LogSetting(config.Config.LogFile)
	
	if useExchange == "bitflyer" {
		fmt.Println(models.DbConnection)	
		controller.StreamIngestionData()
	}
	if useExchange == "okex" {
		fmt.Println(models.DbConnection)
		controller.StartOKEXService()
	}
}