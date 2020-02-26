package controller

import (
	"config"
	"log"
	"math"
	"models"
	"okex"
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
		price01 := roundDecimal(sTf(ticker.Ltp)*0.5 + sTf(ticker.Low)*0.5)
		price02 := roundDecimal(sTf(ticker.Ltp)*0.1 + sTf(ticker.Low)*0.9)
		price03 := roundDecimal(sTf(ticker.Ltp) * 0.97)
		price04 := roundDecimal(sTf(ticker.Ltp) * 0.994)
		log.Printf("#### EOS-USDT price01:%v price02:%v price03:%v", price01, price02, price03)
		placeOkexBuyOrder("EOS-USDT", 0.2, price01, apiClient)
		placeOkexBuyOrder("EOS-USDT", 0.2, price02, apiClient)
		placeOkexBuyOrder("EOS-USDT", 0.2, price03, apiClient)
		placeOkexBuyOrder("EOS-USDT", 0.2, price04, apiClient)
	}

	buyingOKBJob := func() {
		ticker, _ := apiClient.GetOkexTicker("OKB-USDT")
		price01 := roundDecimal(sTf(ticker.Ltp)*0.3 + sTf(ticker.Low)*0.7)
		price02 := roundDecimal(sTf(ticker.Ltp) * 0.972)
		log.Printf("#### OKB-USDT price01:%v price02:%v price03:%v", price01, price02)
		placeOkexBuyOrder("OKB-USDT", 1, price01, apiClient)
		placeOkexBuyOrder("OKB-USDT", 1, price02, apiClient)
	}

	buyingBCHJob := func() {
		ticker, _ := apiClient.GetOkexTicker("BCH-USDT")
		price01 := roundDecimal(sTf(ticker.Ltp)*0.3 + sTf(ticker.Low)*0.7)
		price02 := roundDecimal(sTf(ticker.Ltp) * 0.972)
		log.Printf("#### BCH-USDT price01:%v price02:%v price03:%v", price01, price02)
		placeOkexBuyOrder("BCH-USDT", 0.05, price01, apiClient)
		placeOkexBuyOrder("BCH-USDT", 0.05, price02, apiClient)
	}

	buyingBSVJob := func() {
		ticker, _ := apiClient.GetOkexTicker("BSV-USDT")
		price01 := roundDecimal(sTf(ticker.Ltp)*0.3 + sTf(ticker.Low)*0.7)
		price02 := roundDecimal(sTf(ticker.Ltp) * 0.972)
		log.Printf("#### BSV-USDT price01:%v price02:%v price03:%v", price01, price02)
		placeOkexBuyOrder("BSV-USDT", 0.05, price01, apiClient)
		placeOkexBuyOrder("BSV-USDT", 0.05, price02, apiClient)
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
		scheduler.Every(28800).Seconds().Run(buyingJob)
		scheduler.Every(57600).Seconds().Run(buyingOKBJob)
		scheduler.Every(57600).Seconds().Run(buyingBCHJob)
		scheduler.Every(57600).Seconds().Run(buyingBSVJob)
		scheduler.Every(45).Seconds().Run(placeSellOrderJob)
		scheduler.Every(20).Seconds().Run(syncOrderListJob)
	}
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
			models.UpdateOkexSellOrders(orderID)
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
		OrderType:    "1",
		Price:        fTs(roundDecimal(price)),
		Size:         fTs(size),
	}

	log.Printf("placeOkexOrder:%v\n", order)
	res, err := apiClient.PlaceOrder(order)
	if err != nil {
		log.Println("Buyorder failed.... Failure in [apiClient.PlaceOrder()]")
		return false
	}
	if res == nil {
		log.Println("Buyorder failed.... no response")
		return false
	} else {
		log.Printf("Buyorder response %v", res)
	}
	log.Println("【placeOkexOrder】end of job")
	return true
}
