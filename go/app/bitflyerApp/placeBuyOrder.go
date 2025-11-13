package bitflyerApp

import (
	"fmt"
	"log"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/bitbank"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/bitflyer"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/utils"
)

func placeBuyOrder(strategy int, productCode string, size float64, apiClient *bitflyer.APIClient) {
	log.Printf("strategy:%v", strategy)
	log.Println("【buyingJob】start of job")

	shouldSkip, err := models.ShouldPlaceBuyOrder(apiClient.Max_buy_orders, apiClient.Max_sell_orders)
	if err != nil {
		errMsg := fmt.Sprintf("【ERROR】placeBuyOrder error:%v", err.Error())
		log.Printf("%s\n", errMsg)
		slackClient.PostMessage(errMsg, true)
		log.Println("【buyingJob】end of job as error")
		return
	}
	if shouldSkip {
		log.Printf("ShouldSkip :%v max:%v", shouldSkip, apiClient.Max_sell_orders)
		log.Println("【buyingJob】end of job as skip")
		return
	}

	buyPrice := 0.0
	bitbankClient := bitbank.GetBBTicker("btc_jpy")
	log.Printf("bitbankClient  %v", bitbankClient)

	var res *bitflyer.PlaceOrderResponse
	ticker, _ := apiClient.GetTicker(productCode)

	if strategy < 10 {
		// BTC_JPYの場合
		buyPrice = utils.CalculateBuyPrice(bitbankClient.Last, bitbankClient.Low, strategy)
	} else {
		// ETH_JPYの場合
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
		errMsg := fmt.Sprintf("BuyOrder failed.... Failure in [apiClient.PlaceOrder()] err:%v", err)
		log.Println(errMsg)
		slackClient.PostMessage(errMsg, true)
		log.Println("【buyingJob】end of job as error")
		return
	}

	// Check for API error response
	if res.Status != 0 || res.OrderId == "" {
		var errMsg string
		if res.ErrorMessage != "" {
			errMsg = fmt.Sprintf("BuyOrder failed: %s (Status: %d)", res.ErrorMessage, res.Status)
		} else {
			errMsg = fmt.Sprintf("BuyOrder failed: No order ID returned (Status: %d)", res.Status)
		}
		log.Println(errMsg)
		slackClient.PostMessage(errMsg, true)
		log.Println("【buyingJob】end of job as error")
		return
	}

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
		errMsg := fmt.Sprintf("BuyOrder failed.... Failure in [event.BuyOrder()] err:%v", err)
		log.Printf("%s", errMsg)
		slackClient.PostMessage(errMsg, true)
		log.Println("【buyingJob】end of job as error")
		return
	} else {
		log.Printf("BuyOrder Succeeded! OrderId:%v", res.OrderId)
	}

	slackClient.PostMessage(fmt.Sprintf("BuyOrder: %s(%.2f/%v) OrderId:%v", productCode, buyPrice, size, res.OrderId), true)

	log.Println("【buyingJob】end of job")
}
