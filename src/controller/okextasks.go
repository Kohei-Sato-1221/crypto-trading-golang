package controller

import (
	"config"
	"log"
	"math"
	"models"
	"okex"
	"runtime"
	"strconv"
	"time"

	"github.com/carlescere/scheduler"
	//"runtime"
)

func StartOKEXService() {
	log.Println("【StartOKEXService】")
	apiClient := okex.New(config.Config.OKApiKey, config.Config.OKApiSecret, config.Config.OKPassPhrase)

	buyingJob01 := func() {
		ticker, _ := apiClient.GetOkexTicker("EOS-USDT")
		price := roundDecimal(sTf(ticker.Ltp)*0.4 + sTf(ticker.Low)*0.6)
		log.Printf("#### EOS-USDT price:%v ", price)
		placeOkexBuyOrder("EOS-USDT", 4, price, apiClient)
	}

	buyingJob02 := func() {
		ticker, _ := apiClient.GetOkexTicker("EOS-USDT")
		price := roundDecimal(sTf(ticker.Ltp)*0.1 + sTf(ticker.Low)*0.9)
		log.Printf("#### EOS-USDT price:%v ", price)
		placeOkexBuyOrder("EOS-USDT", 2, price, apiClient)
	}

	buyingJob03 := func() {
		ticker, _ := apiClient.GetOkexTicker("EOS-USDT")
		price := roundDecimal(sTf(ticker.Ltp) * 0.975)
		log.Printf("#### EOS-USDT price:%v ", price)
		placeOkexBuyOrder("EOS-USDT", 3, price, apiClient)
	}

	buyingJob04 := func() {
		ticker, _ := apiClient.GetOkexTicker("EOS-USDT")
		price := roundDecimal(sTf(ticker.Ltp) * 0.985)
		log.Printf("#### EOS-USDT price:%v ", price)
		placeOkexBuyOrder("EOS-USDT", 2, price, apiClient)
	}

	buyingOKBJob01 := func() {
		ticker, _ := apiClient.GetOkexTicker("OKB-USDT")
		price := roundDecimal(sTf(ticker.Ltp)*0.3 + sTf(ticker.Low)*0.7)
		log.Printf("#### OKB-USDT price:%v", price)
		placeOkexBuyOrder("OKB-USDT", 2, price, apiClient)
	}

	buyingOKBJob02 := func() {
		ticker, _ := apiClient.GetOkexTicker("OKB-USDT")
		price := roundDecimal(sTf(ticker.Ltp) * 0.975)
		log.Printf("#### OKB-USDT price:%v", price)
		placeOkexBuyOrder("OKB-USDT", 4, price, apiClient)
	}

	buyingOKBJob03 := func() {
		ticker, _ := apiClient.GetOkexTicker("OKB-USDT")
		price := roundDecimal(sTf(ticker.Ltp) * 0.985)
		log.Printf("#### OKB-USDT price:%v", price)
		placeOkexBuyOrder("OKB-USDT", 2, price, apiClient)
	}

	buyingBCHJob01 := func() {
		ticker, _ := apiClient.GetOkexTicker("BCH-USDT")
		price := roundDecimal(sTf(ticker.Ltp)*0.4 + sTf(ticker.Low)*0.6)
		log.Printf("#### BCH-USDT price:%v ", price)
		placeOkexBuyOrder("BCH-USDT", 0.04, price, apiClient)
	}

	buyingBCHJob02 := func() {
		ticker, _ := apiClient.GetOkexTicker("BCH-USDT")
		price := roundDecimal(sTf(ticker.Ltp)*0.1 + sTf(ticker.Low)*0.9)
		log.Printf("#### BCH-USDT price:%v ", price)
		placeOkexBuyOrder("BCH-USDT", 0.04, price, apiClient)
	}

	buyingBCHJob03 := func() {
		ticker, _ := apiClient.GetOkexTicker("BCH-USDT")
		price := roundDecimal(sTf(ticker.Ltp) * 0.975)
		log.Printf("#### BCH-USDT price:%v ", price)
		placeOkexBuyOrder("BCH-USDT", 0.04, price, apiClient)
	}

	buyingBCHJob04 := func() {
		ticker, _ := apiClient.GetOkexTicker("BCH-USDT")
		price := roundDecimal(sTf(ticker.Ltp) * 0.985)
		log.Printf("#### BCH-USDT price:%v ", price)
		placeOkexBuyOrder("BCH-USDT", 0.04, price, apiClient)
	}

	buyingBSVJob01 := func() {
		ticker, _ := apiClient.GetOkexTicker("BSV-USDT")
		price := roundDecimal(sTf(ticker.Ltp)*0.4 + sTf(ticker.Low)*0.6)
		log.Printf("#### BSV-USDT price:%v ", price)
		placeOkexBuyOrder("BSV-USDT", 0.06, price, apiClient)
	}

	buyingBSVJob02 := func() {
		ticker, _ := apiClient.GetOkexTicker("BSV-USDT")
		price := roundDecimal(sTf(ticker.Ltp)*0.1 + sTf(ticker.Low)*0.9)
		log.Printf("#### BSV-USDT price:%v ", price)
		placeOkexBuyOrder("BSV-USDT", 0.06, price, apiClient)
	}

	buyingBSVJob03 := func() {
		ticker, _ := apiClient.GetOkexTicker("BSV-USDT")
		price := roundDecimal(sTf(ticker.Ltp) * 0.975)
		log.Printf("#### BSV-USDT price:%v ", price)
		placeOkexBuyOrder("BSV-USDT", 0.06, price, apiClient)
	}

	buyingBSVJob04 := func() {
		ticker, _ := apiClient.GetOkexTicker("BSV-USDT")
		price := roundDecimal(sTf(ticker.Ltp) * 0.985)
		log.Printf("#### BSV-USDT price:%v ", price)
		placeOkexBuyOrder("BSV-USDT", 0.06, price, apiClient)
	}

	buyingBTCJob01 := func() {
		ticker, _ := apiClient.GetOkexTicker("BTC-USDT")
		price := roundDecimal(sTf(ticker.Ltp)*0.3 + sTf(ticker.Low)*0.7)
		log.Printf("#### BTC-USDT price:%v ", price)
		placeOkexBuyOrder("BTC-USDT", 0.006, price, apiClient)
	}

	buyingBTCJob02 := func() {
		ticker, _ := apiClient.GetOkexTicker("BTC-USDT")
		price := roundDecimal(sTf(ticker.Ltp) * 0.985)
		log.Printf("#### BTC-USDT price:%v ", price)
		placeOkexBuyOrder("BTC-USDT", 0.006, price, apiClient)
	}

	buyingBTCJob03 := func() {
		ticker, _ := apiClient.GetOkexTicker("BTC-USDT")
		price := roundDecimal(sTf(ticker.Ltp) * 0.997)
		log.Printf("#### BTC-USDT price:%v ", price)
		placeOkexBuyOrder("BTC-USDT", 0.005, price, apiClient)
	}

	buyingETHJob01 := func() {
		ticker, _ := apiClient.GetOkexTicker("ETH-USDT")
		price := roundDecimal(sTf(ticker.Ltp) * 0.995)
		log.Printf("#### ETH-USDT price:%v ", price)
		placeOkexBuyOrder("ETH-USDT", 0.1, price, apiClient)
	}

	buyingETHJob02 := func() {
		ticker, _ := apiClient.GetOkexTicker("ETH-USDT")
		price := roundDecimal(sTf(ticker.Ltp) * 0.98)
		log.Printf("#### ETH-USDT price:%v ", price)
		placeOkexBuyOrder("ETH-USDT", 0.1, price, apiClient)
	}

	buyingETHJob03 := func() {
		ticker, _ := apiClient.GetOkexTicker("ETH-USDT")
		price := roundDecimal(sTf(ticker.Ltp) * 0.97)
		log.Printf("#### ETH-USDT price:%v ", price)
		placeOkexBuyOrder("ETH-USDT", 0.2, price, apiClient)
	}

	placeSellOrderJob := func() {
		log.Println("【placeSellOrderJob】start of job")
		placeSellOrders("EOS-USDT", "EOS", apiClient)
		placeSellOrders("OKB-USDT", "OKB", apiClient)
		placeSellOrders("BCH-USDT", "BCH", apiClient)
		placeSellOrders("BSV-USDT", "BSV", apiClient)
		placeSellOrders("BTC-USDT", "BTC", apiClient)
		placeSellOrders("ETH-USDT", "ETH", apiClient)
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
		shouldSkip := syncOrderList("EOS-USDT", "0", apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
		shouldSkip = syncOrderList("EOS-USDT", "2", apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
		shouldSkip = syncOrderList("OKB-USDT", "0", apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
		shouldSkip = syncOrderList("OKB-USDT", "2", apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
		shouldSkip = syncOrderList("BCH-USDT", "0", apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
		shouldSkip = syncOrderList("BCH-USDT", "2", apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
		shouldSkip = syncOrderList("BSV-USDT", "0", apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
		shouldSkip = syncOrderList("BSV-USDT", "2", apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
		shouldSkip = syncOrderList("BTC-USDT", "0", apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
		shouldSkip = syncOrderList("BTC-USDT", "2", apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
		shouldSkip = syncOrderList("ETH-USDT", "0", apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
		shouldSkip = syncOrderList("ETH-USDT", "2", apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
	ENDOFSELLORDER:
		log.Println("【syncOrderListJob】End of job")
	}

	isTest := false
	smallRunnning := false
	if !isTest {
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

	}
	runtime.Goexit()
}

func syncOrderList(productCode, state string, apiClient *okex.APIClient) bool {
	orders, _ := apiClient.GetOrderList(productCode, state)
	if orders == nil {
		log.Println("【syncOrderListJob】】 : No order ids ")
		return false
	}
	var orderEvents []models.OkexOrderEvent
	utc, _ := time.LoadLocation("UTC")
	utc_current_date := time.Now().In(utc)
	for _, order := range *orders {
		if order.Side == "buy" {
			event := models.OkexOrderEvent{
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
	models.SyncOkexBuyOrders(&orderEvents)
	return true
}

func syncSellOrderList(productCode string, apiClient *okex.APIClient) bool {
	orders, _ := apiClient.GetOrderList(productCode, "2")
	if orders == nil {
		log.Println("【syncSellOrderList】】 : No order ids ")
		return false
	}
	var orderEvents []models.OkexOrderEvent
	utc, _ := time.LoadLocation("UTC")
	utc_current_date := time.Now().In(utc)
	for _, order := range *orders {
		if order.Side == "sell" {
			event := models.OkexOrderEvent{
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
	models.SyncOkexSellOrders(&orderEvents)
	return true
}

func placeSellOrders(pair, currency string, apiClient *okex.APIClient) bool {
	filledBuyOrders := models.GetSoldBuyOrderList(pair)
	available := getAvailableBalance(currency, apiClient)
	if filledBuyOrders == nil {
		log.Println("【placeSellOrderJob】 : No order ids ")
		return false
	}
	for _, buyOrder := range filledBuyOrders {
		orderID := buyOrder.OrderID
		price := buyOrder.Price * 1.015
		size := buyOrder.Size

		log.Printf("placeSellOrder size:%v available:%v ", size, available)
		if size > available {
			size = available
			log.Printf("* available is smaller than size!")
		}
		log.Printf("placeSellOrder orderID:%v pair:%v placeSize:%v price:%v", orderID, pair, size, price)
		sellOrderId := placeOkexSellOrder(orderID, pair, size, price, apiClient)

		if sellOrderId == "" {
			log.Println("placeSellOrder failed.... Failure in [placeSellOrders]")
		} else {
			models.UpdateOkexSellOrders(orderID, sellOrderId, price)
		}
	}
	return true
}

func getAvailableBalance(currency string, apiClient *okex.APIClient) float64 {
	balance, _ := apiClient.GetBlance(currency)
	if balance == nil {
		return 0.00
	} else {
		return sTf(balance.Available)
	}
}

func sTf(str string) float64 {
	f64, error := strconv.ParseFloat(str, 64)
	if error != nil {
		return 0.00
	}
	return f64
}

func fTs(f64 float64) string {
	str := strconv.FormatFloat(f64, 'f', 3, 64)
	return str
}

func roundDecimal(num float64) float64 {
	return math.Round(num*100) / 100
}

func placeOkexBuyOrder(productCode string, size, price float64, apiClient *okex.APIClient) string {
	return placeOkexOrder("buy", "SugarBuyOrder", productCode, size, price, apiClient)
}

func placeOkexSellOrder(orderID, productCode string, size, price float64, apiClient *okex.APIClient) string {
	return placeOkexOrder("sell", "SugarSell"+orderID, productCode, size, price, apiClient)
}

func placeOkexOrder(side, clientOid, productCode string, size, price float64, apiClient *okex.APIClient) string {
	log.Println("【placeOkexOrder】start of job")
	order := &okex.Order{
		ClientOid:    clientOid,
		Type:         "limit",
		Side:         side,
		InstrumentID: productCode,
		OrderType:    "0",
		Price:        fTs(roundDecimal(price)),
		Size:         fTs(size),
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
	} else if res.ErrorCode == "33017" {
		log.Printf("Place Order(1) response %v %v", res.ErrorCode, res.ErrorMsg)
		order.Size = fTs(roundDecimal(size * 0.5))
		res2, err2 := apiClient.PlaceOrder(order)
		if res2 == nil || err2 != nil {
			log.Println("Place Order(2) failed.... no response")
			return ""
		} else if res.ErrorCode != "0" {
			log.Printf("Place Order(2) failed.... bad response %v %v", res.ErrorCode, res.ErrorMsg)
			return ""
		}
	} else if res.ErrorCode != "0" {
		log.Println("Place Order(1) failed.... bad response")
		return ""
	}
	log.Println("【placeOkexOrder】end of job")
	return res.OrderId
}
