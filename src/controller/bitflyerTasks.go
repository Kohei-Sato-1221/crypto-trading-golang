package controller

import (
	"bitflyer"
	"config"
	"models"
	"github.com/carlescere/scheduler"
	"log"
	"time"
	"runtime"
	"bitbank"
	"utils"
)

func StartBfService() {
	log.Println("【StartBfService】start")
	var tickerChannl = make(chan bitflyer.Ticker)
	apiClient := bitflyer.New(
		config.Config.ApiKey,
		config.Config.ApiSecret,
		config.Config.BFMaxSell,
		config.Config.BFMaxBuy,
	)
	go apiClient.GetRealTimeTicker(config.Config.ProductCode, tickerChannl)
	
	
	buyingJob := func(){
		placeBuyOrder(0, apiClient)
	}
	
	buyingJob02 := func(){
		placeBuyOrder(1, apiClient)
	}
	
//	buyingJob03 := func(){
//		placeBuyOrder(-1, apiClient)
//	}
	
	filledCheckJob := func(){
		log.Println("【filledCheckJob】start of job")
		// Get list of unfilled buy orders in local Database(buy_orders & sell_orders)
		ids, err1 := models.FilledCheck()
		if err1 != nil{
			log.Println("error in filledCheckJob.....")
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
				log.Println("error in filledCheckJob.....")
				break
			}
			
			if order != nil{
				err := models.UpdateFilledOrder(orderId)
				if err != nil {
					log.Println("Failure to update records.....")
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
		// get list of orderis whose filled param equqls "1"
		idprices := models.FilledCheckWithSellOrder()
		if idprices == nil{
			log.Println("【sellOrderjob】 : No order ids ")
			goto ENDOFSELLORDER
		}
		
		for i, idprice := range idprices {
			orderId := idprice.OrderId
			buyprice := idprice.Price
			log.Printf("No%d Id:%v", i, orderId)
			sellPrice :=  utils.Round((buyprice * 1.015))
			log.Printf("buyprice:%10.2f  myPrice:%10.2f", buyprice, sellPrice)

			sellOrder := &bitflyer.Order{
				ProductCode:     config.Config.ProductCode,
				ChildOrderType:  "LIMIT",
				Side:            "SELL",
				Price:           sellPrice,
				Size:            0.001,
				MinuteToExpires: 518400, //360 days 
				TimeInForce:     "GTC",
			}
			
			log.Printf("sell order:%v\n", sellOrder)
			res, err := apiClient.PlaceOrder(sellOrder)
			if err != nil{
				log.Println("SellOrder failed.... Failure in [apiClient.PlaceOrder()]")
				break
			}
			if res == nil{
				log.Println("SellOrder failed.... no response")
				break
			}
			
			err = models.UpdateFilledOrderWithBuyOrder(orderId)
			if err != nil {
				log.Println("Failure to update records..... / #UpdateFilledOrderWithBuyOrder")
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
	
	syncBuyOrderJob := func(){
		log.Println("【syncBuyOrderJob】Start of job")
		cnt := models.DeleteStrangeBuyOrderRecords();
		log.Printf("DELETE strange buy_order records :  %v rows deleted", cnt)
		
		orders, err := apiClient.GetActiveOrders()
		if err != nil{
			log.Println("GetActiveOrders failed....")
		}
		var orderEvents []models.OrderEvent
		for _, order := range *orders {
			event := models.OrderEvent{
				OrderId:     order.ChildOrderAcceptanceID,
				Time:        time.Now(),
				ProductCode: order.ProductCode,
				Side:        order.Side,
				Price:       order.Price,
				Size:        order.Size,
				Exchange:    "bitflyer",
			}
			orderEvents = append(orderEvents, event)
			log.Printf("【order】%v", event)
		}
		models.SyncBuyOrders(&orderEvents)
	}
	
	cancelBuyOrderJob := func(){
		log.Println("【cancelBuyOrderJob】Start of job")
		noNeedToCancal := "NoNeedToCancel"
		orderid := models.DetermineCancelledOrder(apiClient.Max_buy_orders, noNeedToCancal);
		log.Printf(" id : %v", orderid)
		var err error
		
		order := &bitflyer.Order{
			ProductCode:     "BTC_JPY",
			ChildOrderAcceptanceID:  orderid,
		}
		
		if orderid == noNeedToCancal {
			goto ENDOFCENCELORDER
		}
		
		err = apiClient.CancelOrder(order)
		if err == nil{
			log.Printf("## Successfully canceled order!! orderid:%v", orderid)
			err = models.UpdateCancelledOrder(orderid)
			if err != nil {
				log.Println("Failure to update records.....")
			}
		}else{
			log.Printf("## Failed to cancel order.... orderid:%v", orderid)			
		}
		
		ENDOFCENCELORDER:
			log.Println("【cancelBuyOrderJob】End of job")
	}
	
	isTest := false
	if !isTest {
//		scheduler.Every(30).Seconds().Run(buyingJob03)
//		scheduler.Every(43200).Seconds().Run(buyingJob)
		
		scheduler.Every().Day().At("05:55").Run(buyingJob)
		scheduler.Every().Day().At("13:05").Run(buyingJob02)
		scheduler.Every().Day().At("17:55").Run(buyingJob)
		scheduler.Every().Day().At("19:55").Run(cancelBuyOrderJob)
		scheduler.Every(45).Seconds().Run(sellOrderJob)
		scheduler.Every(45).Seconds().Run(filledCheckJob)
		scheduler.Every(20).Seconds().Run(syncBuyOrderJob)
	}
	runtime.Goexit()
}

func placeBuyOrder(strategy int, apiClient *bitflyer.APIClient){
	log.Printf("strategy:%v", strategy)
	log.Println("【buyingJob】start of job")
	shouldSkip := models.ShouldPlaceBuyOrder(apiClient.Max_buy_orders, apiClient.Max_sell_orders)
	
	// for test
	if strategy == -1 {
		shouldSkip = false
	}
	log.Printf("ShouldSkip :%v max:%v", shouldSkip, apiClient.Max_sell_orders)
	
	buyPrice := 0.0
	var res *bitflyer.PlaceOrderResponse
	var err error
	
	bitbankClient := bitbank.GetBBTicker()
	log.Printf("bitbankClient  %f", bitbankClient)
	
	// for test 
	// shouldSkip = false
	//
	if !shouldSkip{
		ticker, _ := apiClient.GetTicker("BTC_JPY")
		
		buyPrice = utils.CalculateBuyPrice(bitbankClient.Last, bitbankClient.Low, strategy)
		log.Printf("LTP:%10.2f  BestBid:%10.2f  myPrice:%10.2f", ticker.Ltp, ticker.BestBid, buyPrice)
		
		order := &bitflyer.Order{
			ProductCode:     "BTC_JPY",
			ChildOrderType:  "LIMIT",
			Side:            "BUY",
			Price:           buyPrice,
			Size:            0.001,
			MinuteToExpires: 518400, //360 days 
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





