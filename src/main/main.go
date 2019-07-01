package main

import (
	"fmt"
	"config"
	"utils"
//	"log"
	"bitflyer"
)

func main(){
	utils.LogSetting(config.Config.LogFile)
	apiClient := bitflyer.New(config.Config.ApiKey, config.Config.ApiSecret)
	fmt.Println(apiClient.GetBalance())
//	log.Println("test test")
//	fmt.Println(config.Config.ApiKey)
//	fmt.Println(config.Config.ApiSecret)
}
