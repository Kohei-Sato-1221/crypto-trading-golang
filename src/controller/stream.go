package controller

import (
	"strconv"
	"bitflyer"
	"config"
	"fmt"
	"github.com/carlescere/scheduler"
	"log"
)

func StreamIngestionData() {
	var tickerChannl = make(chan bitflyer.Ticker)
	apiClient := bitflyer.New(config.Config.ApiKey, config.Config.ApiSecret)
	go apiClient.GetRealTimeTicker(config.Config.ProductCode, tickerChannl)
	
	buyingJob := func(){
		ticker, _ := apiClient.GetTicker("BTC_JPY")
		log.Printf("BTC price :%s", strconv.FormatFloat(ticker.GetMiddlePrice(), 'f', 4, 64))
	}
	
	sellingJob := func(){
		fmt.Println("sell")
	}
	
	scheduler.Every(10).Seconds().Run(buyingJob)
	scheduler.Every(5).Seconds().Run(sellingJob)
}