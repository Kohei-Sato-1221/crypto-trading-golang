package app

import (
	"log"
	"runtime"
	"strings"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/bitbank"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/bitflyer"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/config"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/enums"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/models"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/slack"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/utils"
	"github.com/carlescere/scheduler"
)

func StartBfService() {
	log.Println("【StartBfService】start")
	apiClient := bitflyer.NewBitflyer(
		config.Config.ApiKey,
		config.Config.ApiSecret,
		config.Config.BFMaxSell,
		config.Config.BFMaxBuy,
	)

	slackClient := slack.NewSlack(
		config.Config.SlackToken,
		"C01HQKSTK5G",
		"C01M257KX1C",
	)

	postSlackJob := func() {
		sendSlackMessage(slackClient, apiClient)
	}

	buyingJob := func() {
		placeBuyOrder(enums.Stg0BtcLtp3low7, "BTC_JPY", config.Config.BFBTCBuyAmount01, apiClient)
	}

	buyingJob02 := func() {
		placeBuyOrder(enums.Stg1BtcLtp997, "BTC_JPY", config.Config.BFBTCBuyAmount02, apiClient)
	}

	buyingJob03 := func() {
		placeBuyOrder(enums.Stg2BtcLtp98, "BTC_JPY", config.Config.BFBTCBuyAmount03, apiClient)
	}

	buyingJob99 := func() {
		placeBuyOrder(enums.Stg3BtcLtp90, "BTC_JPY", config.Config.BFBTCBuyAmount03, apiClient)
	}

	buyingETHJob := func() {
		placeBuyOrder(enums.Stg10EthLtp995, "ETH_JPY", config.Config.BFETHBuyAmount01, apiClient)
	}

	buyingETHJob02 := func() {
		placeBuyOrder(enums.Stg11EthLtp98, "ETH_JPY", config.Config.BFETHBuyAmount02, apiClient)
	}

	buyingETHJob03 := func() {
		placeBuyOrder(enums.Stg12EthLtp97, "ETH_JPY", config.Config.BFETHBuyAmount03, apiClient)
	}

	buyingETHJob04 := func() {
		placeBuyOrder(enums.Stg13EthLtp3low7, "ETH_JPY", config.Config.BFETHBuyAmount03, apiClient)
	}

	buyingETHJob99 := func() {
		placeBuyOrder(enums.Stg14EthLtp90, "ETH_JPY", config.Config.BFETHBuyAmount03, apiClient)
	}

	btcFilledCheckJob := func() {
		filledCheckJob("BTC_JPY", apiClient)
	}

	ethFilledCheckJob := func() {
		filledCheckJob("ETH_JPY", apiClient)
	}

	sellOrderJob := func() {
		log.Println("【sellOrderjob】start of job")
		// get list of orderis whose filled param equqls "1"
		buyOrderInfos := models.CheckFilledBuyOrders()
		if buyOrderInfos == nil {
			log.Println("【sellOrderjob】 : No order ids ")
			goto ENDOFSELLORDER
		}

		for i, buyOrderInfo := range buyOrderInfos {
			orderID := buyOrderInfo.OrderID
			productCode := buyOrderInfo.ProductCode
			size := buyOrderInfo.Size
			sellPrice := buyOrderInfo.CalculateSellOrderPrice()
			log.Printf("No%d Id:%v sellPrice:%10.2f strategy:%v", i, orderID, sellPrice, buyOrderInfo.Strategy)

			sellOrder := &bitflyer.Order{
				ProductCode:     productCode,
				ChildOrderType:  "LIMIT",
				Side:            "SELL",
				Price:           sellPrice,
				Size:            size,
				MinuteToExpires: 43200, //30days
				TimeInForce:     "GTC",
			}

			log.Printf("sell order:%v\n", sellOrder)
			res, err := apiClient.PlaceOrder(sellOrder)
			log.Printf("sell res:%v\n", res)
			if err != nil {
				log.Println("SellOrder failed.... Failure in [apiClient.PlaceOrder()]")
				break
			}
			if res.OrderId == "" {
				log.Println("SellOrder failed.... no response")
				break
			}

			err = models.UpdateFilledOrderWithBuyOrder(orderID)
			if err != nil {
				log.Println("Failure to update records..... / #UpdateFilledOrderWithBuyOrder")
				break
			}
			log.Printf("Buy Order updated successfully!! #UpdateFilledOrderWithBuyOrder  orderId:%s", orderID)

			utc, _ := time.LoadLocation("UTC")
			utcCurrentDate := time.Now().In(utc)
			event := models.OrderEvent{
				OrderID:     res.OrderId,
				Time:        utcCurrentDate,
				ProductCode: productCode,
				Side:        "Sell",
				Price:       sellPrice,
				Size:        size,
				Exchange:    "bitflyer",
			}
			err = event.SellOrder(orderID)
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
		buyOrders, err := models.GetCancelledBuyOrders()

		if err != nil {
			log.Printf("## failed to cancel order....")
			goto ENDOFCENCELORDER
		}

		for i, order := range buyOrders {
			log.Printf("## %v %v", i, order.OrderID)
			timestamp, err := time.Parse(layout, order.Timestamp)
			if err != nil {
				log.Printf("## failed to cancel order....")
				goto ENDOFCENCELORDER
			}
			cancelCriteria := time.Now().AddDate(0, 0, bfCancelCriteria)

			if cancelCriteria.After(timestamp) {
				cancelOrderParam := &bitflyer.Order{
					ProductCode:            order.ProductCode,
					ChildOrderAcceptanceID: order.OrderID,
				}
				apiClient.CancelOrder(cancelOrderParam)
				models.UpdateCancelledBuyOrder(order.OrderID)
				log.Printf("### %v is cancelled!!", order.OrderID)
			}
		}

	ENDOFCENCELORDER:
		log.Println("【cancelBuyOrderJob】End of job")
	}

	if !config.Config.IsTest {
		scheduler.Every().Day().At("06:30").Run(postSlackJob)
		scheduler.Every(45).Seconds().Run(sellOrderJob)
		scheduler.Every(40).Seconds().Run(syncBTCBuyOrderJob)
		scheduler.Every(40).Seconds().Run(syncETHBuyOrderJob)

		//scheduler.Every().Day().At("00:55").Run(buyingJob)
		//scheduler.Every().Day().At("02:55").Run(buyingJob02)
		//scheduler.Every().Day().At("04:55").Run(buyingJob03)
		//scheduler.Every().Day().At("06:55").Run(buyingJob)
		//scheduler.Every().Day().At("08:55").Run(buyingJob02)
		//scheduler.Every().Day().At("10:55").Run(buyingJob)
		//scheduler.Every().Day().At("12:55").Run(buyingJob02)
		//scheduler.Every().Day().At("14:55").Run(buyingJob03)
		//scheduler.Every().Day().At("16:55").Run(buyingJob)
		//scheduler.Every().Day().At("18:55").Run(buyingJob02)
		//scheduler.Every().Day().At("20:55").Run(buyingJob)
		//scheduler.Every().Day().At("22:55").Run(buyingJob02)
		//scheduler.Every().Day().At("00:05").Run(buyingJob03)

		//scheduler.Every().Day().At("00:53").Run(buyingETHJob)
		//scheduler.Every().Day().At("02:53").Run(buyingETHJob02)
		//scheduler.Every().Day().At("04:53").Run(buyingETHJob03)
		//scheduler.Every().Day().At("06:53").Run(buyingETHJob)
		//scheduler.Every().Day().At("08:53").Run(buyingETHJob02)
		//scheduler.Every().Day().At("10:53").Run(buyingETHJob)
		//scheduler.Every().Day().At("12:53").Run(buyingETHJob02)
		//scheduler.Every().Day().At("14:53").Run(buyingETHJob03)
		//scheduler.Every().Day().At("16:53").Run(buyingETHJob)
		//scheduler.Every().Day().At("18:53").Run(buyingETHJob02)
		//scheduler.Every().Day().At("20:53").Run(buyingETHJob)
		//scheduler.Every().Day().At("22:53").Run(buyingETHJob02)
		//scheduler.Every().Day().At("00:03").Run(buyingETHJob03)

		// Start 20210513修正
		scheduler.Every().Day().At("02:55").Run(buyingJob)
		scheduler.Every().Day().At("06:55").Run(buyingJob02)
		scheduler.Every().Day().At("10:55").Run(buyingJob03)
		scheduler.Every().Day().At("14:55").Run(buyingJob)
		scheduler.Every().Day().At("18:55").Run(buyingJob02)
		scheduler.Every().Day().At("22:55").Run(buyingJob03)

		scheduler.Every().Day().At("01:53").Run(buyingETHJob)
		scheduler.Every().Day().At("04:53").Run(buyingETHJob02)
		scheduler.Every().Day().At("07:53").Run(buyingETHJob03)
		scheduler.Every().Day().At("10:53").Run(buyingETHJob04)
		scheduler.Every().Day().At("13:53").Run(buyingETHJob)
		scheduler.Every().Day().At("16:53").Run(buyingETHJob02)
		scheduler.Every().Day().At("19:53").Run(buyingETHJob03)
		scheduler.Every().Day().At("22:53").Run(buyingETHJob04)

		scheduler.Every().Day().At("00:53").Run(buyingJob99)
		scheduler.Every().Day().At("00:55").Run(buyingETHJob99)
		// End

		scheduler.Every(45).Seconds().Run(ethFilledCheckJob)
		scheduler.Every(45).Seconds().Run(btcFilledCheckJob)
		scheduler.Every(7200).Seconds().Run(deleteRecordJob)

		scheduler.Every().Day().At("23:45").Run(cancelBuyOrderJob)
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
				OrderID:     order.ChildOrderAcceptanceID,
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
	// Completedされた注文に関しては2日以内に約定した注文のみ同期
	for _, order := range *completed_orders {
		utc, _ := time.LoadLocation("UTC")
		utc_current_date := time.Now().In(utc)
		compareOrderDate, _ := time.ParseInLocation("2006-01-02 15:04:05", strings.Replace(order.ChildOrderDate, "T", " ", 1), time.UTC)
		compareOrderDate = compareOrderDate.Add(60 * time.Minute)
		//compareOrderDate = compareOrderDate.Add(2880 * time.Minute)
		if order.Side == "BUY" && compareOrderDate.After(utc_current_date) {
			event := models.OrderEvent{
				OrderID:     order.ChildOrderAcceptanceID,
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

func filledCheckJob(productCode string, apiClient *bitflyer.APIClient) {
	log.Println("【filledCheckJob】start of job %v", productCode)
	// Get list of unfilled buy orders in local Database(buy_orders & sell_orders)
	ids, err1 := models.FilledCheck(productCode)
	completed_orders, err2 := apiClient.GetActiveBuyOrders(productCode, "COMPLETED")
	if err1 != nil || err2 != nil {
		log.Println("error in filledCheckJob..... e1:%v  e2:%v", err1, err2)
		goto ENDOFFILLEDCHECK
	}

	if ids == nil {
		goto ENDOFFILLEDCHECK
	}

	// check if an order is filled for each orders calling API
	for i, orderId := range ids {
		log.Printf("No%d Id:%v", i, orderId)
		// order, err := apiClient.GetOrderByOrderId(orderId, productCode)
		orderIdExist := false
		for _, order := range *completed_orders {
			if orderId == order.ChildOrderAcceptanceID {
				orderIdExist = true
				log.Printf("## filledCheckJob  orderid:%v has been filled!")
				break
			}
		}
		if orderIdExist {
			err := models.UpdateFilledOrder(orderId)
			if err != nil {
				log.Println("Failure to update records.....")
				break
			}
			log.Printf("Order updated successfully!! orderId:%s", orderId)
		}
	}
ENDOFFILLEDCHECK:
	log.Println("【filledCheckJob】end of job %v", productCode)

}

func placeBuyOrder(strategy int, productCode string, size float64, apiClient *bitflyer.APIClient) {
	log.Printf("strategy:%v", strategy)
	log.Println("【buyingJob】start of job")
	//shouldSkip, err := models.ShouldPlaceBuyOrder(apiClient.Max_buy_orders, apiClient.Max_sell_orders)
	//if err != nil {
	//	log.Printf("【ERROR】placeBuyOrder error:%v", err.Error())
	//	return
	//}

	// for test
	//if strategy == -1 {
	//	shouldSkip = false
	//}
	shouldSkip := false

	log.Printf("ShouldSkip :%v max:%v", shouldSkip, apiClient.Max_sell_orders)

	buyPrice := 0.0
	bitbankClient := bitbank.GetBBTicker("btc_jpy")
	log.Printf("bitbankClient  %f", bitbankClient)

	var res *bitflyer.PlaceOrderResponse
	var err error
	if !shouldSkip {
		ticker, _ := apiClient.GetTicker(productCode)

		if strategy < 10 {
			//BTC_JPYの場合
			buyPrice = utils.CalculateBuyPrice(bitbankClient.Last, bitbankClient.Low, strategy)
		} else {
			//ETH_JPYの場合
			buyPrice = utils.CalculateBuyPrice(ticker.Ltp, ticker.BestBid, strategy)
		}

		minuteToExpire := models.CalculateMinuteToExpire(strategy)
		log.Printf("LTP:%10.2f  BestBid:%10.2f  myPrice:%10.2f minuteToExpire:%v", ticker.Ltp, ticker.BestBid, buyPrice, minuteToExpire)

		order := &bitflyer.Order{
			ProductCode:     productCode,
			ChildOrderType:  "LIMIT",
			Side:            "BUY",
			Price:           buyPrice,
			Size:            size,
			MinuteToExpires: minuteToExpire,
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
			OrderID:     res.OrderId,
			Time:        utc_current_date,
			ProductCode: productCode,
			Side:        "BUY",
			Price:       buyPrice,
			Size:        size,
			Exchange:    "bitflyer",
			Strategy:    strategy,
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

func sendSlackMessage(client *slack.APIClient, apiClient *bitflyer.APIClient) error {
	log.Println("【sendSlackMessage】start of job")
	text, err := models.GetResults()
	if err != nil {
		return err
	}
	err = client.PostMessage(text, false)
	if err != nil {
		return err
	}
	log.Println("【sendSlackMessage】end of job")
	return nil
}
