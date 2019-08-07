package controller

import (
	"bitflyer"
	"config"
	"models"
	"github.com/carlescere/scheduler"
	"log"
	"time"
	"math"
	"runtime"
)

func StreamIngestionData() {
	log.Println("【StreamIngestionData】start")
	var tickerChannl = make(chan bitflyer.Ticker)
	apiClient := bitflyer.New(config.Config.ApiKey, config.Config.ApiSecret)
	numParallelOrders := config.Config.ParallelOrders
	go apiClient.GetRealTimeTicker(config.Config.ProductCode, tickerChannl)
	
	buyingJob := func(){
		log.Println("【buyingJob】start of job")
		cnt := models.CountUnfilledBuyOrders()
		log.Printf("Number of unfilled :%v  parall:%v", cnt, numParallelOrders)
		shouldSkip := false
		if cnt >= numParallelOrders {
			shouldSkip = true
		}
		
		buyPrice := 0.0
		var res *bitflyer.PlaceOrderResponse
		var err error
		if !shouldSkip{
			ticker, _ := apiClient.GetTicker("BTC_JPY")
			
			buyPrice =  Round((ticker.Ltp * 0.4 + ticker.BestBid * 0.6))
			log.Printf("LTP:%10.2f  BestBid:%10.2f  myPrice:%10.2f", ticker.Ltp, ticker.BestBid, buyPrice)
			
			order := &bitflyer.Order{
				ProductCode:     "BTC_JPY",
				ChildOrderType:  "LIMIT",
				Side:            "BUY",
				Price:           buyPrice,
				Size:            0.001,
				MinuteToExpires: 1000,
				TimeInForce:     "GTC",
			}
			res, err = apiClient.PlaceOrder(order)
			if err != nil || res == nil {
				log.Println("BuyOrder failed.... Failure in [apiClient.PlaceOrder()]")
				shouldSkip = true
			}
		}
		if !shouldSkip {
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
			}else{
				log.Printf("BuyOrder Succeeded! OrderId:%v", res.OrderId)			
			}
		}
		log.Println("【buyingJob】end of job")
	}
	
	filledCheckJob := func(){
		log.Println("【filledCheckJob】start of job")
		// Get list of unfilled buy orders in local Database
		ids, err1 := models.FilledCheck()
		if err1 != nil{
			log.Fatal("error in filledCheckJob.....")
			goto ENDOFFILLEDCHECK
		}
		
		if ids == nil{
			goto ENDOFFILLEDCHECK
		}
		
		// check if an order is filled for each orders calling API
		for i, orderId := range ids {
			log.Printf("No%d Id:%v", i, orderId)
			order, err := apiClient.GetOrderByOrderId(orderId)
			if err != nil{
				log.Fatal("error in filledCheckJob.....")
				break
			}
			
			if order != nil{
				err := models.UpdateFilledOrder(orderId)
				if err != nil {
					log.Fatal("Failure to update records.....")
					break
				}
				log.Printf("Order updated successfully!! orderId:%s", orderId)								
			}
		}
		ENDOFFILLEDCHECK:
			log.Println("【filledCheckJob】end of job")
	}
	
	sellOrderJob := func(){
		log.Println("【sellOrderjob】start of job")
		idprices := models.FilledCheckWithSellOrder()
		if idprices == nil{
			log.Println("【sellOrderjob】 : No order ids ")
			goto ENDOFSELLORDER
		}
		
		for i, idprice := range idprices {
			orderId := idprice.OrderId
			buyprice := idprice.Price
			log.Printf("No%d Id:%v", i, orderId)
			sellPrice :=  Round((buyprice * 1.005))
			log.Printf("buyprice:%10.2f  myPrice:%10.2f", buyprice, sellPrice)

			sellOrder := &bitflyer.Order{
				ProductCode:     config.Config.ProductCode,
				ChildOrderType:  "LIMIT",
				Side:            "SELL",
				Price:           sellPrice,
				Size:            0.001,
				MinuteToExpires: 1000,
				TimeInForce:     "GTC",
			}
			
			log.Printf("sell order:%v\n", sellOrder)
			res, err := apiClient.PlaceOrder(sellOrder)
			if err != nil{
				log.Fatal("SellOrder failed.... Failure in [apiClient.PlaceOrder()]")
				break
			}
			if res == nil{
				log.Fatal("SellOrder failed.... no response")
			}
			
			err = models.UpdateFilledOrderWithBuyOrder(orderId)
			if err != nil {
				log.Fatal("Failure to update records..... / #UpdateFilledOrderWithBuyOrder")
				break
			}
			log.Printf("Buy Order updated successfully!! #UpdateFilledOrderWithBuyOrder  orderId:%s", orderId)
			
			event := models.OrderEvent{
				OrderId:     res.OrderId,
				Time:        time.Now(),
				ProductCode: "BTC_JPY",
				Side:        "Sell",
				Price:       sellPrice,
				Size:        0.001,
				Exchange:    "bitflyer",
			}
			err = event.SellOrder(orderId)
			if err != nil{
				log.Println("BuyOrder failed.... Failure in [event.BuyOrder()]")
			}else{
				log.Printf("BuyOrder Succeeded! OrderId:%v", res.OrderId)			
			}
		}
		ENDOFSELLORDER:
			log.Println("【sellOrderjob】end of job")
	}
	testFlg := true
	if(testFlg){
		scheduler.Every(60).Seconds().Run(buyingJob)
		scheduler.Every(20).Seconds().Run(sellOrderJob)
	} 
	scheduler.Every(10).Seconds().Run(filledCheckJob)
	runtime.Goexit()
}

func Round(f float64) float64{
	return math.Floor(f + .5) 
}




