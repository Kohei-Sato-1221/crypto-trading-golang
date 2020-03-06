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

	buyingJob := func() {
		ticker, _ := apiClient.GetOkexTicker("EOS-USDT")
		price01 := roundDecimal(sTf(ticker.Ltp)*0.4 + sTf(ticker.Low)*0.6)
		price02 := roundDecimal(sTf(ticker.Ltp)*0.1 + sTf(ticker.Low)*0.9)
		price03 := roundDecimal(sTf(ticker.Ltp) * 0.975)
		price04 := roundDecimal(sTf(ticker.Ltp) * 0.985)
		log.Printf("#### EOS-USDT price01:%v price02:%v price03:%v", price01, price02, price03)
		placeOkexBuyOrder("EOS-USDT", 2, price01, apiClient)
		placeOkexBuyOrder("EOS-USDT", 2, price02, apiClient)
		placeOkexBuyOrder("EOS-USDT", 2, price03, apiClient)
		placeOkexBuyOrder("EOS-USDT", 2, price04, apiClient)
	}

	buyingOKBJob := func() {
		ticker, _ := apiClient.GetOkexTicker("OKB-USDT")
		price01 := roundDecimal(sTf(ticker.Ltp)*0.3 + sTf(ticker.Low)*0.7)
		price02 := roundDecimal(sTf(ticker.Ltp) * 0.975)
		price03 := roundDecimal(sTf(ticker.Ltp) * 0.985)
		log.Printf("#### OKB-USDT price01:%v price02:%v price03:%v", price01, price02)
		placeOkexBuyOrder("OKB-USDT", 2, price01, apiClient)
		placeOkexBuyOrder("OKB-USDT", 2, price02, apiClient)
		placeOkexBuyOrder("OKB-USDT", 2, price03, apiClient)
	}

	buyingBCHJob := func() {
		ticker, _ := apiClient.GetOkexTicker("BCH-USDT")
		price01 := roundDecimal(sTf(ticker.Ltp)*0.4 + sTf(ticker.Low)*0.6)
		price02 := roundDecimal(sTf(ticker.Ltp)*0.1 + sTf(ticker.Low)*0.9)
		price03 := roundDecimal(sTf(ticker.Ltp) * 0.975)
		price04 := roundDecimal(sTf(ticker.Ltp) * 0.985)
		log.Printf("#### BCH-USDT price01:%v price02:%v price03:%v", price01, price02)
		placeOkexBuyOrder("BCH-USDT", 0.02, price01, apiClient)
		placeOkexBuyOrder("BCH-USDT", 0.02, price02, apiClient)
		placeOkexBuyOrder("BCH-USDT", 0.02, price03, apiClient)
		placeOkexBuyOrder("BCH-USDT", 0.02, price04, apiClient)
	}

	buyingBSVJob := func() {
		ticker, _ := apiClient.GetOkexTicker("BSV-USDT")
		price01 := roundDecimal(sTf(ticker.Ltp)*0.4 + sTf(ticker.Low)*0.6)
		price02 := roundDecimal(sTf(ticker.Ltp)*0.1 + sTf(ticker.Low)*0.9)
		price03 := roundDecimal(sTf(ticker.Ltp) * 0.975)
		price04 := roundDecimal(sTf(ticker.Ltp) * 0.985)
		log.Printf("#### BSV-USDT price01:%v price02:%v price03:%v", price01, price02)
		placeOkexBuyOrder("BSV-USDT", 0.03, price01, apiClient)
		placeOkexBuyOrder("BSV-USDT", 0.03, price02, apiClient)
		placeOkexBuyOrder("BSV-USDT", 0.03, price03, apiClient)
		placeOkexBuyOrder("BSV-USDT", 0.03, price04, apiClient)
	}

	placeSellOrderJob := func() {
		log.Println("【placeSellOrderJob】start of job")
		placeSellOrders("EOS-USDT", apiClient)
		placeSellOrders("OKB-USDT", apiClient)
		placeSellOrders("BCH-USDT", apiClient)
		placeSellOrders("BSV-USDT", apiClient)
		log.Println("【placeSellOrderJob】end of job")
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
	ENDOFSELLORDER:
		log.Println("【syncOrderListJob】End of job")
	}

	isTest := false
	if !isTest {
		scheduler.Every(30).Seconds().Run(syncOrderListJob)
		scheduler.Every(55).Seconds().Run(placeSellOrderJob)
		scheduler.Every().Day().At("05:55").Run(buyingJob)
		scheduler.Every().Day().At("05:55").Run(buyingOKBJob)
		scheduler.Every().Day().At("05:55").Run(buyingBCHJob)
		scheduler.Every().Day().At("05:55").Run(buyingBSVJob)
		scheduler.Every().Day().At("11:45").Run(buyingJob)
		scheduler.Every().Day().At("11:45").Run(buyingOKBJob)
		scheduler.Every().Day().At("11:45").Run(buyingBCHJob)
		scheduler.Every().Day().At("11:45").Run(buyingBSVJob)
		scheduler.Every().Day().At("19:30").Run(buyingJob)
		scheduler.Every().Day().At("19:30").Run(buyingOKBJob)
		scheduler.Every().Day().At("19:30").Run(buyingBCHJob)
		scheduler.Every().Day().At("19:30").Run(buyingBSVJob)
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
		} else {

		}
	}
	models.SyncOkexBuyOrders(&orderEvents)
	return true
}

func placeSellOrders(pair string, apiClient *okex.APIClient) bool {
	filledBuyOrders := models.GetSoldBuyOrderList(pair)
	if filledBuyOrders == nil {
		log.Println("【placeSellOrderJob】 : No order ids ")
		return false
	}
	for _, buyOrder := range filledBuyOrders {
		orderID := buyOrder.OrderID
		price := buyOrder.Price * 1.015
		size := buyOrder.Size

		log.Printf("placeSellOrder  %v %v %v %v", orderID, pair, size, price)
		shouldSkip := placeOkexSellOrder(orderID, pair, size, price, apiClient)

		if !shouldSkip {
			log.Println("placeSellOrder failed.... Failure in [placeSellOrders]")
		} else {
			models.UpdateOkexSellOrders(orderID, price)
		}
	}
	return true
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

func placeOkexBuyOrder(productCode string, size, price float64, apiClient *okex.APIClient) bool {
	return placeOkexOrder("buy", "SugarBuyOrder", productCode, size, price, apiClient)
}

func placeOkexSellOrder(orderID, productCode string, size, price float64, apiClient *okex.APIClient) bool {
	return placeOkexOrder("sell", "SugarSell"+orderID, productCode, size, price, apiClient)
}

func placeOkexOrder(side, clientOid, productCode string, size, price float64, apiClient *okex.APIClient) bool {
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
		return false
	}
	if res == nil {
		log.Println("Place Order(1) failed.... no response")
		return false
	} else if res.ErrorCode == "33017" {
		log.Printf("Place Order(1) response %v %v", res.ErrorCode, res.ErrorMsg)
		order.Size = fTs(roundDecimal(size * 0.5))
		res2, err2 := apiClient.PlaceOrder(order)
		if res2 == nil || err2 != nil {
			log.Println("Place Order(2) failed.... no response")
			return false
		} else if res.ErrorCode != "0" {
			log.Printf("Place Order(2) failed.... bad response %v %v", res.ErrorCode, res.ErrorMsg)
			return false
		}
	} else if res.ErrorCode != "0" {
		log.Println("Place Order(1) failed.... bad response")
		return false
	}
	log.Println("【placeOkexOrder】end of job")
	return true
}
