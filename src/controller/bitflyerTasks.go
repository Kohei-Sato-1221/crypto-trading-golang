package controller

import (
	"bitbank"
	"bitflyer"
	"config"
	"log"
	"models"
	"runtime"
	"strings"
	"time"
	"utils"

	"github.com/carlescere/scheduler"
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

	buyingJob := func() {
		placeBuyOrder(0, "BTC_JPY", 0.002, apiClient)
	}

	buyingJob02 := func() {
		placeBuyOrder(1, "BTC_JPY", 0.002, apiClient)
	}

	buyingETHJob := func() {
		placeBuyOrder(10, "ETH_JPY", 0.03, apiClient)
	}

	buyingETHJob02 := func() {
		placeBuyOrder(11, "ETH_JPY", 0.03, apiClient)
	}

	//	buyingJob03 := func(){
	//		placeBuyOrder(-1, apiClient)
	//	}

	filledCheckJob := func() {
		log.Println("【filledCheckJob】start of job")
		// Get list of unfilled buy orders in local Database(buy_orders & sell_orders)
		ids, err1 := models.FilledCheck()
		if err1 != nil {
			log.Println("error in filledCheckJob.....")
			goto ENDOFFILLEDCHECK
		}

		if ids == nil {
			goto ENDOFFILLEDCHECK
		}

		// check if an order is filled for each orders calling API
		for i, orderId := range ids {
			log.Printf("No%d Id:%v", i, orderId)
			order, err := apiClient.GetOrderByOrderId(orderId)
			if err != nil {
				log.Println("error in filledCheckJob.....")
				break
			}

			if order != nil {
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

	sellOrderJob := func() {
		log.Println("【sellOrderjob】start of job")
		// get list of orderis whose filled param equqls "1"
		idprices := models.FilledCheckWithSellOrder()
		if idprices == nil {
			log.Println("【sellOrderjob】 : No order ids ")
			goto ENDOFSELLORDER
		}

		for i, idprice := range idprices {
			orderId := idprice.OrderId
			buyprice := idprice.Price
			product_code := idprice.ProductCode
			size := idprice.Size
			log.Printf("No%d Id:%v", i, orderId)
			sellPrice := utils.Round((buyprice * 1.015))
			log.Printf("buyprice:%10.2f  myPrice:%10.2f", buyprice, sellPrice)

			sellOrder := &bitflyer.Order{
				ProductCode:     product_code,
				ChildOrderType:  "LIMIT",
				Side:            "SELL",
				Price:           sellPrice,
				Size:            size,
				MinuteToExpires: 518400, //360 days
				TimeInForce:     "GTC",
			}

			log.Printf("sell order:%v\n", sellOrder)
			res, err := apiClient.PlaceOrder(sellOrder)
			if err != nil {
				log.Println("SellOrder failed.... Failure in [apiClient.PlaceOrder()]")
				break
			}
			if res == nil {
				log.Println("SellOrder failed.... no response")
				break
			}

			err = models.UpdateFilledOrderWithBuyOrder(orderId)
			if err != nil {
				log.Println("Failure to update records..... / #UpdateFilledOrderWithBuyOrder")
				break
			}
			log.Printf("Buy Order updated successfully!! #UpdateFilledOrderWithBuyOrder  orderId:%s", orderId)

			utc, _ := time.LoadLocation("UTC")
			utc_current_date := time.Now().In(utc)
			event := models.OrderEvent{
				OrderId:     res.OrderId,
				Time:        utc_current_date,
				ProductCode: product_code,
				Side:        "Sell",
				Price:       sellPrice,
				Size:        size,
				Exchange:    "bitflyer",
			}
			err = event.SellOrder(orderId)
			if err != nil {
				log.Println("BuyOrder failed.... Failure in [event.BuyOrder()]")
			} else {
				log.Printf("BuyOrder Succeeded! OrderId:%v", res.OrderId)
			}
		}
	ENDOFSELLORDER:
		log.Println("【sellOrderjob】end of job")
	}

	syncBTCBuyOrderJob := func() {
		log.Println("【syncBTCBuyOrderJob】Start of job")
		syncBuyOrders("BTC_JPY", apiClient)
		log.Println("【syncBTCBuyOrderJob】End of job")
	}

	syncETHBuyOrderJob := func() {
		log.Println("【syncETHBuyOrderJob】Start of job")
		syncBuyOrders("ETH_JPY", apiClient)
		log.Println("【syncETHBuyOrderJob】End of job")
	}

	deleteRecordJob := func() {
		log.Println("【deleteRecordJob】Start of job")
		cnt := models.DeleteStrangeBuyOrderRecords()
		log.Printf("DELETE strange buy_order records :  %v rows deleted", cnt)
		log.Println("【deleteRecordJob】End of job")
	}

	cancelBuyOrderJob := func() {
		log.Println("【cancelBuyOrderJob】Start of job")
		noNeedToCancal := "NoNeedToCancel"
		orderid := models.DetermineCancelledOrder(apiClient.Max_buy_orders, noNeedToCancal)
		log.Printf(" id : %v", orderid)
		var err error

		order := &bitflyer.Order{
			ProductCode:            "BTC_JPY",
			ChildOrderAcceptanceID: orderid,
		}

		if orderid == noNeedToCancal {
			goto ENDOFCENCELORDER
		}

		err = apiClient.CancelOrder(order)
		if err == nil {
			log.Printf("## Successfully canceled order!! orderid:%v", orderid)
			err = models.UpdateCancelledOrder(orderid)
			if err != nil {
				log.Println("Failure to update records.....")
			}
		} else {
			log.Printf("## Failed to cancel order.... orderid:%v", orderid)
		}

	ENDOFCENCELORDER:
		log.Println("【cancelBuyOrderJob】End of job")
	}

	isTest := false
	if !isTest {
		scheduler.Every(45).Seconds().Run(sellOrderJob)
		scheduler.Every(20).Seconds().Run(syncETHBuyOrderJob)
		scheduler.Every(20).Seconds().Run(syncBTCBuyOrderJob)
		scheduler.Every().Day().At("05:55").Run(buyingJob)
		scheduler.Every().Day().At("13:05").Run(buyingJob02)
		scheduler.Every().Day().At("17:55").Run(buyingJob)
		scheduler.Every().Day().At("04:55").Run(buyingETHJob)
		scheduler.Every().Day().At("12:05").Run(buyingETHJob02)
		scheduler.Every().Day().At("16:55").Run(buyingETHJob)
		scheduler.Every().Day().At("19:55").Run(cancelBuyOrderJob)
		scheduler.Every(45).Seconds().Run(filledCheckJob)
		scheduler.Every(7200).Seconds().Run(deleteRecordJob)
	}
	runtime.Goexit()
}

func syncBuyOrders(product_code string, apiClient *bitflyer.APIClient) {
	active_orders, err := apiClient.GetActiveBuyOrders(product_code, "ACTIVE")
	completed_orders, err := apiClient.GetActiveBuyOrders(product_code, "COMPLETED")
	if err != nil {
		log.Println("GetActiveOrders failed....")
	}
	var orderEvents []models.OrderEvent
	utc, _ := time.LoadLocation("UTC")
	utc_current_date := time.Now().In(utc)
	for _, order := range *active_orders {
		if order.Side == "BUY" {
			event := models.OrderEvent{
				OrderId:     order.ChildOrderAcceptanceID,
				Time:        utc_current_date,
				ProductCode: order.ProductCode,
				Side:        order.Side,
				Price:       order.Price,
				Size:        order.Size,
				Exchange:    "bitflyer",
				Filled:      0,
			}
			orderEvents = append(orderEvents, event)
			log.Printf("【order】%v", event)
		}
	}
	// Completedされた注文に関しては120分以内に約定した注文のみ同期
	for _, order := range *completed_orders {
		utc, _ := time.LoadLocation("UTC")
		utc_current_date := time.Now().In(utc)
		compareOrderDate, _ := time.ParseInLocation("2006-01-02 15:04:05", strings.Replace(order.ChildOrderDate, "T", " ", 1), time.UTC)
		compareOrderDate = compareOrderDate.Add(240 * time.Minute)
		if order.Side == "BUY" && compareOrderDate.After(utc_current_date) {
			event := models.OrderEvent{
				OrderId:     order.ChildOrderAcceptanceID,
				Time:        utc_current_date,
				ProductCode: order.ProductCode,
				Side:        order.Side,
				Price:       order.Price,
				Size:        order.Size,
				Exchange:    "bitflyer",
				Filled:      1,
			}
			orderEvents = append(orderEvents, event)
			log.Printf("【order】%v", event)
		}
	}
	models.SyncBuyOrders(&orderEvents)
}

func placeBuyOrder(strategy int, productCode string, size float64, apiClient *bitflyer.APIClient) {
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
	if !shouldSkip {
		ticker, _ := apiClient.GetTicker(productCode)

		if strategy < 10 {
			//BTC_JPYの場合
			buyPrice = utils.CalculateBuyPrice(bitbankClient.Last, bitbankClient.Low, strategy)
		} else {
			//ETH_JPYの場合
			buyPrice = utils.CalculateBuyPrice(ticker.Ltp, ticker.BestBid, strategy)
		}
		log.Printf("LTP:%10.2f  BestBid:%10.2f  myPrice:%10.2f", ticker.Ltp, ticker.BestBid, buyPrice)

		order := &bitflyer.Order{
			ProductCode:     productCode,
			ChildOrderType:  "LIMIT",
			Side:            "BUY",
			Price:           buyPrice,
			Size:            size,
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
		utc, _ := time.LoadLocation("UTC")
		utc_current_date := time.Now().In(utc)
		event := models.OrderEvent{
			OrderId:     res.OrderId,
			Time:        utc_current_date,
			ProductCode: productCode,
			Side:        "BUY",
			Price:       buyPrice,
			Size:        size,
			Exchange:    "bitflyer",
		}
		err = event.BuyOrder()
		if err != nil {
			log.Println("BuyOrder failed.... Failure in [event.BuyOrder()]")
		} else {
			log.Printf("BuyOrder Succeeded! OrderId:%v", res.OrderId)
		}
	}
	log.Println("【buyingJob】end of job")
}
