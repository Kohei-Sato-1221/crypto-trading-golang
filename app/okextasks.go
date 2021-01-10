package app

import (
	"log"
	"runtime"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/config"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/okex"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/slack"
	"github.com/carlescere/scheduler"
)

func StartOKEXService(exchange string) {
	log.Println("【StartOKEXService】")
	apiClient := okex.New(
		config.Config.OKApiKey,
		config.Config.OKApiSecret,
		config.Config.OKPassPhrase,
		config.Config.Exchange,
	)

	slackClient := slack.NewSlack(
		config.Config.SlackToken,
		"C01HQKSTK5G",
	)

	postSlackJob := func() {
		sendOKexSlackMessage(slackClient, apiClient)
	}

	buyingJob01 := func() {
		ticker, _ := apiClient.GetOkexTicker("EOS-USDT")
		price := RoundDecimal(STf(ticker.Ltp)*0.4 + STf(ticker.Low)*0.6)
		log.Printf("#### EOS-USDT price:%v ", price)
		placeOkexBuyOrder("EOS-USDT", 4, price, apiClient, slackClient)
	}

	buyingJob02 := func() {
		ticker, _ := apiClient.GetOkexTicker("EOS-USDT")
		price := RoundDecimal(STf(ticker.Ltp)*0.1 + STf(ticker.Low)*0.9)
		log.Printf("#### EOS-USDT price:%v ", price)
		placeOkexBuyOrder("EOS-USDT", 2, price, apiClient, slackClient)
	}

	buyingJob03 := func() {
		ticker, _ := apiClient.GetOkexTicker("EOS-USDT")
		price := RoundDecimal(STf(ticker.Ltp) * 0.975)
		log.Printf("#### EOS-USDT price:%v ", price)
		placeOkexBuyOrder("EOS-USDT", 3, price, apiClient, slackClient)
	}

	buyingJob04 := func() {
		ticker, _ := apiClient.GetOkexTicker("EOS-USDT")
		price := RoundDecimal(STf(ticker.Ltp) * 0.985)
		log.Printf("#### EOS-USDT price:%v ", price)
		placeOkexBuyOrder("EOS-USDT", 2, price, apiClient, slackClient)
	}

	buyingOKBJob01 := func() {
		ticker, _ := apiClient.GetOkexTicker("OKB-USDT")
		price := RoundDecimal(STf(ticker.Ltp)*0.3 + STf(ticker.Low)*0.7)
		log.Printf("#### OKB-USDT price:%v", price)
		placeOkexBuyOrder("OKB-USDT", 2, price, apiClient, slackClient)
	}

	buyingOKBJob02 := func() {
		ticker, _ := apiClient.GetOkexTicker("OKB-USDT")
		price := RoundDecimal(STf(ticker.Ltp) * 0.975)
		log.Printf("#### OKB-USDT price:%v", price)
		placeOkexBuyOrder("OKB-USDT", 4, price, apiClient, slackClient)
	}

	buyingOKBJob03 := func() {
		ticker, _ := apiClient.GetOkexTicker("OKB-USDT")
		price := RoundDecimal(STf(ticker.Ltp) * 0.985)
		log.Printf("#### OKB-USDT price:%v", price)
		placeOkexBuyOrder("OKB-USDT", 2, price, apiClient, slackClient)
	}

	buyingBCHJob01 := func() {
		ticker, _ := apiClient.GetOkexTicker("BCH-USDT")
		price := RoundDecimal(STf(ticker.Ltp)*0.4 + STf(ticker.Low)*0.6)
		log.Printf("#### BCH-USDT price:%v ", price)
		placeOkexBuyOrder("BCH-USDT", 0.04, price, apiClient, slackClient)
	}

	buyingBCHJob02 := func() {
		ticker, _ := apiClient.GetOkexTicker("BCH-USDT")
		price := RoundDecimal(STf(ticker.Ltp)*0.1 + STf(ticker.Low)*0.9)
		log.Printf("#### BCH-USDT price:%v ", price)
		placeOkexBuyOrder("BCH-USDT", 0.04, price, apiClient, slackClient)
	}

	buyingBCHJob03 := func() {
		ticker, _ := apiClient.GetOkexTicker("BCH-USDT")
		price := RoundDecimal(STf(ticker.Ltp) * 0.975)
		log.Printf("#### BCH-USDT price:%v ", price)
		placeOkexBuyOrder("BCH-USDT", 0.04, price, apiClient, slackClient)
	}

	buyingBCHJob04 := func() {
		ticker, _ := apiClient.GetOkexTicker("BCH-USDT")
		price := RoundDecimal(STf(ticker.Ltp) * 0.985)
		log.Printf("#### BCH-USDT price:%v ", price)
		placeOkexBuyOrder("BCH-USDT", 0.04, price, apiClient, slackClient)
	}

	buyingBSVJob01 := func() {
		ticker, _ := apiClient.GetOkexTicker("BSV-USDT")
		price := RoundDecimal(STf(ticker.Ltp)*0.4 + STf(ticker.Low)*0.6)
		log.Printf("#### BSV-USDT price:%v ", price)
		placeOkexBuyOrder("BSV-USDT", 0.06, price, apiClient, slackClient)
	}

	buyingBSVJob02 := func() {
		ticker, _ := apiClient.GetOkexTicker("BSV-USDT")
		price := RoundDecimal(STf(ticker.Ltp)*0.1 + STf(ticker.Low)*0.9)
		log.Printf("#### BSV-USDT price:%v ", price)
		placeOkexBuyOrder("BSV-USDT", 0.06, price, apiClient, slackClient)
	}

	buyingBSVJob03 := func() {
		ticker, _ := apiClient.GetOkexTicker("BSV-USDT")
		price := RoundDecimal(STf(ticker.Ltp) * 0.975)
		log.Printf("#### BSV-USDT price:%v ", price)
		placeOkexBuyOrder("BSV-USDT", 0.06, price, apiClient, slackClient)
	}

	buyingBSVJob04 := func() {
		ticker, _ := apiClient.GetOkexTicker("BSV-USDT")
		price := RoundDecimal(STf(ticker.Ltp) * 0.985)
		log.Printf("#### BSV-USDT price:%v ", price)
		placeOkexBuyOrder("BSV-USDT", 0.06, price, apiClient, slackClient)
	}

	buyingBTCJob01 := func() {
		ticker, _ := apiClient.GetOkexTicker("BTC-USDT")
		price := RoundDecimal(STf(ticker.Ltp)*0.3 + STf(ticker.Low)*0.7)
		log.Printf("#### BTC-USDT price:%v ", price)
		placeOkexBuyOrder("BTC-USDT", 0.007, price, apiClient, slackClient)
	}

	buyingBTCJob02 := func() {
		ticker, _ := apiClient.GetOkexTicker("BTC-USDT")
		price := RoundDecimal(STf(ticker.Ltp) * 0.985)
		log.Printf("#### BTC-USDT price:%v ", price)
		placeOkexBuyOrder("BTC-USDT", 0.007, price, apiClient, slackClient)
	}

	buyingBTCJob03 := func() {
		ticker, _ := apiClient.GetOkexTicker("BTC-USDT")
		price := RoundDecimal(STf(ticker.Ltp) * 0.997)
		log.Printf("#### BTC-USDT price:%v ", price)
		placeOkexBuyOrder("BTC-USDT", 0.006, price, apiClient, slackClient)
	}

	buyingETHJob01 := func() {
		ticker, _ := apiClient.GetOkexTicker("ETH-USDT")
		price := RoundDecimal(STf(ticker.Ltp) * 0.995)
		log.Printf("#### ETH-USDT price:%v ", price)
		placeOkexBuyOrder("ETH-USDT", 0.2, price, apiClient, slackClient)
	}

	buyingETHJob02 := func() {
		ticker, _ := apiClient.GetOkexTicker("ETH-USDT")
		price := RoundDecimal(STf(ticker.Ltp) * 0.98)
		log.Printf("#### ETH-USDT price:%v ", price)
		placeOkexBuyOrder("ETH-USDT", 0.2, price, apiClient, slackClient)
	}

	buyingETHJob03 := func() {
		ticker, _ := apiClient.GetOkexTicker("ETH-USDT")
		price := RoundDecimal(STf(ticker.Ltp) * 0.97)
		log.Printf("#### ETH-USDT price:%v ", price)
		placeOkexBuyOrder("ETH-USDT", 0.3, price, apiClient, slackClient)
	}

	placeSellOrderJob := func() {
		log.Println("【placeSellOrderJob】start of job")
		profitRate := 1.015
		placeSellOrders("EOS-USDT", "EOS", profitRate, apiClient, slackClient)
		placeSellOrders("OKB-USDT", "OKB", profitRate, apiClient, slackClient)
		placeSellOrders("BCH-USDT", "BCH", profitRate, apiClient, slackClient)
		placeSellOrders("BSV-USDT", "BSV", profitRate, apiClient, slackClient)
		placeSellOrders("BTC-USDT", "BTC", profitRate, apiClient, slackClient)
		placeSellOrders("ETH-USDT", "ETH", profitRate, apiClient, slackClient)
		log.Println("【placeSellOrderJob】end of job")
	}

	syncSellOrderListJob := func() {
		log.Println("【syncSellOrderListJob】Start of job")
		shouldSkip := syncSellOrderList("EOS-USDT", apiClient)
		if !shouldSkip {
			goto ENDOFSYNCSELLORDER
		}
		shouldSkip = syncSellOrderList("OKB-USDT", apiClient)
		if !shouldSkip {
			goto ENDOFSYNCSELLORDER
		}
		shouldSkip = syncSellOrderList("BCH-USDT", apiClient)
		if !shouldSkip {
			goto ENDOFSYNCSELLORDER
		}
		shouldSkip = syncSellOrderList("BSV-USDT", apiClient)
		if !shouldSkip {
			goto ENDOFSYNCSELLORDER
		}
		shouldSkip = syncSellOrderList("BTC-USDT", apiClient)
		if !shouldSkip {
			goto ENDOFSYNCSELLORDER
		}
		shouldSkip = syncSellOrderList("ETH-USDT", apiClient)
		if !shouldSkip {
			goto ENDOFSYNCSELLORDER
		}
	ENDOFSYNCSELLORDER:
		log.Println("【syncSellOrderListJob】End of job")
	}

	syncOrderListJob := func() {
		log.Println("【syncOrderListJob】Start of job")
		shouldSkip := syncOrderList("EOS-USDT", "0", exchange, apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
		shouldSkip = syncOrderList("EOS-USDT", "2", exchange, apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
		shouldSkip = syncOrderList("OKB-USDT", "0", exchange, apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
		shouldSkip = syncOrderList("OKB-USDT", "2", exchange, apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
		shouldSkip = syncOrderList("BCH-USDT", "0", exchange, apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
		shouldSkip = syncOrderList("BCH-USDT", "2", exchange, apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
		shouldSkip = syncOrderList("BSV-USDT", "0", exchange, apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
		shouldSkip = syncOrderList("BSV-USDT", "2", exchange, apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
		shouldSkip = syncOrderList("BTC-USDT", "0", exchange, apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
		shouldSkip = syncOrderList("BTC-USDT", "2", exchange, apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
		shouldSkip = syncOrderList("ETH-USDT", "0", exchange, apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
		shouldSkip = syncOrderList("ETH-USDT", "2", exchange, apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
	ENDOFSELLORDER:
		log.Println("【syncOrderListJob】End of job")
	}

	cancelBuyOrderJob := func() {
		log.Println("【cancelBuyOrderJob】Start of job")
		buyOrders, err := okex.GetCancelledOrders()
		cancelCriteria := time.Now().AddDate(0, 0, okexCancelCriteria)

		if err != nil {
			log.Printf("## failed to cancel order....")
			goto ENDOFCENCELORDER
		}

		log.Printf("## cancelCriteria:%v", cancelCriteria)
		for i, order := range buyOrders {
			timestamp, err := time.Parse(layout, order.Timestamp)
			if err != nil {
				log.Printf("## failed to cancel order....")
				goto ENDOFCENCELORDER
			}
			log.Printf("## %v %v timestamp:%v %v %v", i, order.OrderID, order.Timestamp, order.Pair, order.Price)

			if cancelCriteria.After(timestamp) {
				apiClient.CancelOrder(&order)
				okex.UpdateCancelledOrder(order.OrderID)
				log.Printf("### %v is cancelled!!", order.OrderID)
			}
		}
	ENDOFCENCELORDER:
		log.Println("【cancelBuyOrderJob】End of job")
	}

	smallRunnning := false
	if !config.Config.IsTest {
		scheduler.Every().Day().At("06:30").Run(postSlackJob)
		scheduler.Every(30).Seconds().Run(syncOrderListJob)
		scheduler.Every(300).Seconds().Run(syncSellOrderListJob)
		scheduler.Every(55).Seconds().Run(placeSellOrderJob)

		if !smallRunnning {
			scheduler.Every().Day().At("03:55").Run(buyingJob01)
			scheduler.Every().Day().At("09:55").Run(buyingJob02)
			scheduler.Every().Day().At("15:55").Run(buyingJob03)
			scheduler.Every().Day().At("21:55").Run(buyingJob04)

			scheduler.Every().Day().At("02:55").Run(buyingOKBJob01)
			scheduler.Every().Day().At("10:55").Run(buyingOKBJob02)
			scheduler.Every().Day().At("18:55").Run(buyingOKBJob03)

			scheduler.Every().Day().At("05:55").Run(buyingBCHJob01)
			scheduler.Every().Day().At("11:55").Run(buyingBCHJob02)
			scheduler.Every().Day().At("17:55").Run(buyingBCHJob03)
			scheduler.Every().Day().At("23:55").Run(buyingBCHJob04)

			scheduler.Every().Day().At("08:55").Run(buyingBSVJob01)
			scheduler.Every().Day().At("14:55").Run(buyingBSVJob02)
			scheduler.Every().Day().At("20:55").Run(buyingBSVJob03)
			scheduler.Every().Day().At("04:55").Run(buyingBSVJob04)
		}

		scheduler.Every().Day().At("00:30").Run(buyingBTCJob01)
		scheduler.Every().Day().At("04:30").Run(buyingBTCJob02)
		scheduler.Every().Day().At("08:30").Run(buyingBTCJob03)
		scheduler.Every().Day().At("12:30").Run(buyingBTCJob01)
		scheduler.Every().Day().At("16:30").Run(buyingBTCJob02)
		scheduler.Every().Day().At("20:30").Run(buyingBTCJob03)

		scheduler.Every().Day().At("00:40").Run(buyingETHJob01)
		scheduler.Every().Day().At("04:40").Run(buyingETHJob02)
		scheduler.Every().Day().At("08:40").Run(buyingETHJob03)
		scheduler.Every().Day().At("12:40").Run(buyingETHJob01)
		scheduler.Every().Day().At("16:40").Run(buyingETHJob02)
		scheduler.Every().Day().At("20:40").Run(buyingETHJob03)

		scheduler.Every().Day().At("23:45").Run(cancelBuyOrderJob)
	}
	runtime.Goexit()
}

func syncOrderList(productCode, state, exchange string, apiClient *okex.APIClient) bool {
	orders, _ := apiClient.GetOrderList(productCode, state)
	if orders == nil {
		log.Println("【syncOrderListJob】】 : No order ids ")
		return false
	}
	var orderEvents []okex.OkexOrderEvent
	utc, _ := time.LoadLocation("UTC")
	utc_current_date := time.Now().In(utc)
	for _, order := range *orders {
		if order.Side == "buy" {
			event := okex.OkexOrderEvent{
				OrderID:      order.OrderID,
				Timestamp:    utc_current_date,
				InstrumentID: order.InstrumentID,
				Side:         order.Side,
				Price:        order.Price,
				Size:         order.Size,
				State:        order.State,
			}
			orderEvents = append(orderEvents, event)
			log.Printf(" ### pair:%v price:%v size:%v state:%v time:%v", order.InstrumentID, order.Price, order.Size, order.State, order.Timestamp)
		}
	}
	okex.SyncOkexBuyOrders(exchange, &orderEvents)
	return true
}

func syncSellOrderList(productCode string, apiClient *okex.APIClient) bool {
	orders, _ := apiClient.GetOrderList(productCode, "2")
	if orders == nil {
		log.Println("【syncSellOrderList】】 : No order ids ")
		return false
	}
	var orderEvents []okex.OkexOrderEvent
	utc, _ := time.LoadLocation("UTC")
	utc_current_date := time.Now().In(utc)
	for _, order := range *orders {
		if order.Side == "sell" {
			event := okex.OkexOrderEvent{
				OrderID:      order.OrderID,
				Timestamp:    utc_current_date,
				InstrumentID: order.InstrumentID,
				Side:         order.Side,
				Price:        order.Price,
				Size:         order.Size,
				State:        order.State,
			}
			orderEvents = append(orderEvents, event)
			log.Printf(" ### pair:%v price:%v size:%v state:%v time:%v", order.InstrumentID, order.Price, order.Size, order.State, order.Timestamp)
		}
	}
	okex.SyncOkexSellOrders(&orderEvents)
	return true
}

func placeSellOrders(pair, currency string, profitRate float64, apiClient *okex.APIClient, slackClient *slack.APIClient) bool {
	filledBuyOrders := okex.GetSoldBuyOrderList(pair)
	available := GetAvailableBalance(currency, apiClient)
	if filledBuyOrders == nil {
		log.Println("【placeSellOrderJob】 : No order ids ")
		return false
	}
	for _, buyOrder := range filledBuyOrders {
		orderID := buyOrder.OrderID
		// price := buyOrder.Price * 1.015
		price := buyOrder.Price * profitRate
		size := buyOrder.Size

		log.Printf("placeSellOrder size:%v available:%v ", size, available)
		if size > available {
			size = available
			log.Printf("* available is smaller than size!")
		}
		log.Printf("placeSellOrder orderID:%v pair:%v placeSize:%v price:%v", orderID, pair, size, price)
		sellOrderId := placeOkexSellOrder(orderID, pair, size, price, apiClient, slackClient)

		if sellOrderId == "" {
			log.Println("placeSellOrder failed.... Failure in [placeSellOrders]")
		} else {
			okex.UpdateOkexSellOrders(orderID, sellOrderId, price)
		}
	}
	return true
}

func GetAvailableBalance(currency string, apiClient *okex.APIClient) float64 {
	balance, _ := apiClient.GetBlance(currency)
	if balance == nil {
		return 0.00
	} else {
		return STf(balance.Available)
	}
}

func placeOkexBuyOrder(productCode string, size, price float64, apiClient *okex.APIClient, slackClinet *slack.APIClient) string {
	return placeOkexOrder("buy", "SugarBuyOrder", productCode, size, price, apiClient, slackClinet)
}

func placeOkexSellOrder(orderID, productCode string, size, price float64, apiClient *okex.APIClient, slackClinet *slack.APIClient) string {
	return placeOkexOrder("sell", "SugarSell"+orderID, productCode, size, price, apiClient, slackClinet)
}

func placeOkexOrder(side, clientOid, productCode string, size, price float64, apiClient *okex.APIClient, slackClient *slack.APIClient) string {
	log.Println("【placeOkexOrder】start of job")
	order := &okex.Order{
		ClientOid:    clientOid,
		Type:         "limit",
		Side:         side,
		InstrumentID: productCode,
		OrderType:    "0",
		Price:        FTs(RoundDecimal(price)),
		Size:         FTs(size),
	}

	log.Printf("placeOkexOrder:%v\n", order)
	res, err := apiClient.PlaceOrder(order)
	if err != nil {
		log.Println("Place Order(1) failed.... Failure in [apiClient.PlaceOrder()]")
		return ""
	}
	if res == nil {
		log.Println("Place Order(1) failed.... no response")
		return ""
	} else if len(res.ErrorCode) > 0 {
		text := getErrorMessageForSlack(
			res.ErrorCode,
			res.ErrorMsg,
			side,
			productCode,
			FTs(RoundDecimal(price)),
			FTs(size))

		if side == "sell" && res.ErrorCode == "33017" {
			text += "## Application's Terminated!! ##"
			slackClient.PostMessage(text)
			runtime.Goexit()
		} else {
			slackClient.PostMessage(text)
		}
	} else if res.ErrorCode != "0" {
		log.Println("Place Order(1) failed.... bad response")
		return ""
	}
	log.Println("【placeOkexOrder】end of job")
	return res.OrderId
}

func getErrorMessageForSlack(errorCode, errorMsg, side, code, price, size string) string {
	return "## ERROR CODE:" + errorCode + " " + errorMsg + "\n" +
		"EXCHANGE:" + config.Config.Exchange + "\n" +
		"SIDE:" + side + "\n" +
		"PAIR:" + code + "\n" +
		"Price:" + price + "\n" +
		"Size:" + size + "\n"
}

func sendOKexSlackMessage(client *slack.APIClient, apiClient *okex.APIClient) error {
	log.Println("【sendOKexSlackMessage】start of job")
	text, err := okex.GetOKexResults()
	if err != nil {
		return err
	}
	err = client.PostMessage(text)
	if err != nil {
		return err
	}
	log.Println("【sendOKexSlackMessage】end of job")
	return nil
}
