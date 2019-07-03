package main

import (
	"fmt"
	"config"
	"utils"
	"time"
//	"log"
	"bitflyer"
)

func main(){
	utils.LogSetting(config.Config.LogFile)
	apiClient := bitflyer.New(config.Config.ApiKey, config.Config.ApiSecret)
	
	tickerChan := make(chan bitflyer.Ticker)
	go apiClient.GetRealTimeTicker(config.Config.ProductCode, tickerChan)
	for ticker := range tickerChan {
		fmt.Println(ticker)
		fmt.Println(ticker.GetMiddlePrice())
		fmt.Println(ticker.DateTime())
		fmt.Println(ticker.TruncateDateTime(time.Second))
		fmt.Println(ticker.TruncateDateTime(time.Minute))
		fmt.Println(ticker.TruncateDateTime(time.Hour))
	}
	
//	ticker, _ := apiCli?ent.GetTicker("BTC_USD")
//	fmt.Println(ticker)
//	fmt.Println(ticker.GetMiddlePrice())
//	fmt.Println(ticker.DateTime())
//	fmt.Println(ticker.TruncateDateTime(time.Hour))
//	fmt.Println(apiClient.GetBalance())
//	log.Println("test test")
//	fmt.Println(config.Config.ApiKey)
//	fmt.Println(config.Config.ApiSecret)
}
