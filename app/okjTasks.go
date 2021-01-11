package app

import (
	"log"
	"runtime"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/slack"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/bitbank"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/config"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/okex"
	"github.com/carlescere/scheduler"
)

func StartOKJService(exchange string) {
	log.Println("【StartOKJService】")
	apiClient := okex.New(
		config.Config.OKJApiKey,
		config.Config.OKJApiSecret,
		config.Config.OKJPassPhrase,
		config.Config.Exchange,
	)

	slackClient := slack.NewSlack(
		config.Config.SlackToken,
		"C01HQKSTK5G",
	)

	buyingJob01 := func() {
		bbClient := bitbank.GetBBTicker("btc_jpy")
		prices := getBuyPrices(bbClient.Low, bbClient.Last, 6)
		for _, price := range prices {
			log.Printf("#### BTC-JPY price:%v ", price)
			placeOkexBuyOrder("BTC-JPY", 0.002, price, apiClient, slackClient)
		}
	}

	buyingJob02 := func() {
		bbClient := bitbank.GetBBTicker("eth_jpy")
		prices := getBuyPrices(bbClient.Low, bbClient.Last, 6)
		for _, price := range prices {
			log.Printf("#### ETH-JPY price:%v ", price)
			placeOkexBuyOrder("ETH-JPY", 0.04, price, apiClient, slackClient)
		}
	}

	placeSellOrderJob := func() {
		log.Println("【placeSellOrderJob】start of job")
		profitRate := 1.018
		placeSellOrders("BTC-JPY", "BTC", profitRate, apiClient, slackClient)
		placeSellOrders("ETH-JPY", "ETH", profitRate, apiClient, slackClient)
		log.Println("【placeSellOrderJob】end of job")
	}

	syncSellOrderListJob := func() {
		log.Println("【syncSellOrderListJob】Start of job")
		shouldSkip := syncSellOrderList("BTC-JPY", apiClient)
		if !shouldSkip {
			goto ENDOFSYNCSELLORDER
		}
		shouldSkip = syncSellOrderList("ETH-JPY", apiClient)
		if !shouldSkip {
			goto ENDOFSYNCSELLORDER
		}
	ENDOFSYNCSELLORDER:
		log.Println("【syncSellOrderListJob】End of job")
	}

	syncOrderListJob := func() {
		log.Println("【syncOrderListJob】Start of job")
		shouldSkip := syncOrderList("BTC-JPY", "0", exchange, apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
		shouldSkip = syncOrderList("BTC-JPY", "2", exchange, apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
		shouldSkip = syncOrderList("ETH-JPY", "0", exchange, apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
		shouldSkip = syncOrderList("ETH-JPY", "2", exchange, apiClient)
		if !shouldSkip {
			goto ENDOFSELLORDER
		}
	ENDOFSELLORDER:
		log.Println("【syncOrderListJob】End of job")
	}

	cancelBuyOrderJob := func() {
		log.Println("【cancelBuyOrderJob】Start of job")
		buyOrders, err := okex.GetOKJCancelledOrders()
		cancelCriteria := time.Now().AddDate(0, 0, okjCancelCriteria)

		if err != nil {
			log.Printf("## failed to cancel order....")
			goto ENDOFCENCELORDER
		}

		log.Printf("## cancelCriteria:%v", cancelCriteria)
		for i, order := range buyOrders {
			if err != nil {
				log.Printf("## failed to cancel order....")
				goto ENDOFCENCELORDER
			}
			log.Printf("## %v %v timestamp:%v %v %v", i, order.OrderID, order.Timestamp, order.Pair, order.Price)

			apiClient.CancelOrder(order.OrderID, order.Pair)
			okex.UpdateCancelledOrder(order.OrderID)
			log.Printf("### %v is cancelled!!", order.OrderID)
		}
	ENDOFCENCELORDER:
		log.Println("【cancelBuyOrderJob】End of job")
	}

	if !config.Config.IsTest {
		scheduler.Every(30).Seconds().Run(syncOrderListJob)
		scheduler.Every(300).Seconds().Run(syncSellOrderListJob)
		scheduler.Every(55).Seconds().Run(placeSellOrderJob)

		scheduler.Every().Day().At("11:18").Run(cancelBuyOrderJob)
		scheduler.Every().Day().At("11:20").Run(buyingJob01)
		scheduler.Every().Day().At("11:22").Run(buyingJob02)

	}
	runtime.Goexit()
}

func getBuyPrices(low, last float64, numOfPrices int) []float64 {
	roundedLow := RoundDecimal(low * 1.005)
	roundedLast := RoundDecimal(last * 0.995)

	diff := (roundedLast - roundedLow) / float64(numOfPrices)
	prices := make([]float64, 0, numOfPrices+1)

	for i := 0; i < numOfPrices+1; i++ {
		prices = append(prices, RoundDecimal(roundedLow+(diff*float64(i))))
	}

	return prices
}
