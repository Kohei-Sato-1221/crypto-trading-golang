package controller

import (
//	"strconv"
	"bitflyer"
	"config"
//	"fmt"
	"models"
	"github.com/carlescere/scheduler"
	"log"
	"time"
)

func StreamIngestionData() {
	var tickerChannl = make(chan bitflyer.Ticker)
	apiClient := bitflyer.New(config.Config.ApiKey, config.Config.ApiSecret)
	go apiClient.GetRealTimeTicker(config.Config.ProductCode, tickerChannl)
	
	buyingJob := func(){
		ticker, _ := apiClient.GetTicker("BTC_JPY")
		
		buyPrice :=  (ticker.Ltp * 0.6 + ticker.BestBid * 0.4)
		log.Printf("LTP:%10.2f  BestBid:%10.2f  myPrice:%10.2f", ticker.Ltp, ticker.BestBid, buyPrice)
		
		order := &bitflyer.Order{
			ProductCode:     config.Config.ProductCode,
			ChildOrderType:  "LIMIT",
			Side:            "BUY",
			Price:           buyPrice,
			Size:            0.001,
			MinuteToExpires: 1000,
			TimeInForce:     "GTC",
		}
		res, err := apiClient.PlaceOrder(order)
		if err != nil{
			log.Println("BuyOrder failed.... Failure in [apiClient.PlaceOrder()]")
			return
		}
		
		event := models.OrderEvent{
			OrderId:     res.OrderId,
			Time:        time.Now(),
			ProductCode: "BTC_JPY",
			Side:        "BUY",
			Price:       buyPrice,
			Size:        0.001,
			Exchange:    "bitflyer",
		}
		err = event.BuyOrder()
		if err != nil{
			log.Println("BuyOrder failed.... Failure in [event.BuyOrder()]")
			return
		}else{
			log.Printf("BuyOrder Succeeded! OrderId:%v", res.OrderId)			
		}
	}
	
	filledCheckJob := func(){
		// Get list of unfilled buy orders in local Database
		ids := models.FilledCheck()
		if ids == nil{
			log.Fatal("error in filledCheckJob.....")
			return
		}
		
		// check if an order is filled for each orders calling API
		for i, orderId := range ids {
			log.Printf("No%d.Id:%v", i, orderId)
			order, err := apiClient.GetOrderByOrderId(orderId)
			if err != nil{
				log.Fatal("error in filledCheckJob.....")
				return
			}
			
			if order != nil{
				err := models.UpdateFilledOrder(orderId)
				if err != nil {
					log.Fatal("Failure to update records.....")
					return
				}
				log.Printf("Order updated successfully!! orderId:%s", orderId)								
			}
		}
	}
	
	sellOrderJob := func(){
		ticker, _ := apiClient.GetTicker("BTC_JPY")
		ids := models.FilledCheckWithSellOrder()
		if ids == nil{
			log.Fatal("error in filledCheckJob.....")
			return
		}
		
		for i, orderId := range ids {
			log.Printf("No%d.Id:%v", i, orderId)
			sellPrice :=  (ticker.Ltp * 1.01)
			log.Printf("LTP:%10.2f  myPrice:%10.2f", ticker.Ltp, sellPrice)
		
			sellOrder := &bitflyer.Order{
				ProductCode:     config.Config.ProductCode,
				ChildOrderType:  "LIMIT",
				Side:            "BUY",
				Price:           sellPrice,
				Size:            0.001,
				MinuteToExpires: 1000,
				TimeInForce:     "GTC",
			}
			res, _ := apiClient.PlaceOrder(sellOrder)
			if res != nil{
				log.Println("SellOrder failed.... Failure in [apiClient.PlaceOrder()]")
				return
			}
			
			if sellOrder != nil{
				err := models.UpdateFilledOrderWithBuyOrder(orderId)
				if err != nil {
					log.Fatal("Failure to update records..... / #UpdateFilledOrderWithBuyOrder")
					return
				}
				log.Printf("Order updated successfully!! #UpdateFilledOrderWithBuyOrder  orderId:%s", orderId)								
			}
		}
	}
	
	testFlg := false
	if(testFlg){
		scheduler.Every(10).Seconds().Run(buyingJob)		
		scheduler.Every(1000).Seconds().Run(filledCheckJob)
	} 
	scheduler.Every(1000).Seconds().Run(sellOrderJob)
}

/*
【jobの種類】

1.buyOrderJob:
・指定の周期で買い注文を発注するジョブ
・買い注文が発注したら以下のデータをinsertする
　[Table:buyorder]orderid, pair, volume, price, orderdate, exchange, filled 

2.filledCheckJob:
・指定の周期で買い注文の約定具合をチェックするジョブ
・買い注文が約定していた場合、buyorderテーブルのfilledをtrueにする

3.sellOrderJob:
・指定の周期で売り注文を発注するジョブ
・buyoerderのレコードでfilledがtrueかつ、sellorderに該当のorderidがない場合売り注文を出す。
・売り注文が発注できたら以下のデータをinsertする。
　[Table:sellorder]buyorderid, orderid, pair, volume, price, orderdate, exchange, filled 


*/




