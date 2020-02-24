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
		price03 := roundDecimal(sTf(ticker.Ltp) * 0.97)
		log.Printf("#### EOS-USDT price01:%v price02:%v price03:%v", price01, price02, price03)
		placeOkexOrder("buy", "EOS-USDT", 0.2, price01, apiClient)
		placeOkexOrder("buy", "EOS-USDT", 0.2, price02, apiClient)
		placeOkexOrder("buy", "EOS-USDT", 0.2, price03, apiClient)
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
	ENDOFSELLORDER:
		log.Println("【syncOrderListJob】End of job")
	}

	isTest := true
	scheduler.Every(45).Seconds().Run(syncOrderListJob)
	if !isTest {
		scheduler.Every(45).Seconds().Run(buyingJob)
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
		event := models.OkexOrderEvent{
			OrderId:      order.OrderId,
			Timestamp:    utc_current_date,
			InstrumentId: order.InstrumentId,
			Side:         order.Side,
			Price:        order.Price,
			Size:         order.Size,
			State:        order.State,
		}
		orderEvents = append(orderEvents, event)
		log.Printf(" ### pair:%v price:%v size:%v state:%v time:%v", order.InstrumentId, order.Price, order.Size, order.State, order.Timestamp)
	}
	models.SyncOkexBuyOrders(&orderEvents)
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

func placeOkexOrder(side, productCode string, size, price float64, apiClient *okex.APIClient) {
	log.Println("【buyingJob】start of job")
	buyOrder := &okex.Order{
		ClientOid:    "SugarTrading",
		Type:         "limit",
		Side:         side,
		InstrumentId: productCode,
		OrderType:    "1",
		Price:        fTs(price),
		Size:         fTs(size),
	}

	log.Printf("buyorder:%v\n", buyOrder)
	res, err := apiClient.PlaceOrder(buyOrder)
	if err != nil {
		log.Println("Buyorder failed.... Failure in [apiClient.PlaceOrder()]")
		return
	}
	if res == nil {
		log.Println("Buyorder failed.... no response")
		return
	} else {
		log.Println("Buyorder response %v %s", res)
	}
	log.Println("【buyingJob】end of job")
}
