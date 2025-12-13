package bitflyerApp

import (
	"fmt"
	"log"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/bitbank"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/bitflyer"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/config"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/enums"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/utils"
)

func placeBuyOrder(strategy int, productCode string, size float64, apiClient *bitflyer.APIClient, weekday *string) {
	log.Printf("strategy:%v", strategy)
	log.Println("【buyingJob】start of job")

	// 今日が指定された曜日でなければスキップする
	if weekday != nil && !enums.IsTodayWeekday(*weekday) {
		log.Printf("【buyingJob】Skipped!! Today is not %s\n", *weekday)
		return
	}

	// 日本円の残高を調べて、BudgetCriteria未満であればスキップする
	jpyBalance, err := apiClient.GetJPYBalance()
	if err != nil {
		errMsg := fmt.Sprintf("【ERROR】Failed to get JPY balance: %v", err)
		log.Printf("%s\n", errMsg)
		slackClient.PostMessage(errMsg, true)
		log.Println("【buyingJob】end of job as error")
		return
	}
	if jpyBalance < config.Config.BudgetCriteria {
		msg := fmt.Sprintf("【buyingJob】Skipped!! JPY balance (%.2f) is below BudgetCriteria (%.2f)", jpyBalance, config.Config.BudgetCriteria)
		log.Println(msg)
		log.Println("【buyingJob】end of job as skip")
		slackClient.PostMessage(msg, false)
		return
	}
	log.Printf("【buyingJob】JPY balance: %.2f (BudgetCriteria: %.2f)", jpyBalance, config.Config.BudgetCriteria)

	// 最大注文数を超えている場合はスキップする
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

	// 過去7日間の最低価格を取得
	var lowestPriceInPast7Days *float64
	if strategy >= 20001 {
		lowestPriceInPast7Days, _ = models.GetLowestPriceInPast7Days(productCode)
	}

	if strategy < 10 {
		// BTC_JPYの場合
		buyPrice = utils.CalculateBuyPrice(bitbankClient.Last, bitbankClient.Low, strategy, lowestPriceInPast7Days)
	} else {
		// ETH_JPYの場合
		buyPrice = utils.CalculateBuyPrice(ticker.Ltp, ticker.BestBid, strategy, lowestPriceInPast7Days)
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
