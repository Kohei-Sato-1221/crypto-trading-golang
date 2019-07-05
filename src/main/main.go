package main

import (
	"fmt"
	"config"
	"utils"
	"models"
//	"time"
//	"log"
//	"bitflyer"
)

func main(){
	utils.LogSetting(config.Config.LogFile)
	fmt.Println(models.DbConnection)
//	apiClient := bitflyer.New(config.Config.ApiKey, config.Config.ApiSecret)
	
//	order := &bitflyer.Order{
//	    ProductCode     : config.Config.ProductCode,
//		ChildOrderType  : "LIMIT",
//		Side            : "BUY",
//		Price           : 1000000,
//		Size            : 0.001,
//		MinuteToExpires : 1,
//		TimeInForce     : "GTC",
//	}
//	res, _ := apiClient.PlaceOrder(order)
//	fmt.Println(res.ChildOrderAcceptanceID)
//	

//	params := map[string]string{
//		"product_code"              : config.Config.ProductCode,
//		"child_order_acceptance_id" : "JRF20190704-131322-262181",
//	}
//	r, _ := apiClient.GetOrderInfo(params)
//	fmt.Println(r)
	
//	tickerChan := make(chan bitflyer.Ticker)
//	go apiClient.GetRealTimeTicker(config.Config.ProductCode, tickerChan)
//	for ticker := range tickerChan {
//		fmt.Println(ticker)
//		fmt.Println(ticker.GetMiddlePrice())
//		fmt.Println(ticker.DateTime())
//		fmt.Println(ticker.TruncateDateTime(time.Second))
//		fmt.Println(ticker.TruncateDateTime(time.Minute))
//		fmt.Println(ticker.TruncateDateTime(time.Hour))
//	}
	
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
