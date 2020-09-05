package controller

import (
	"config"
	"log"
	"okex"
	"runtime"

	"github.com/carlescere/scheduler"
	//"runtime"
)

func StartOKJService(exchange string) {
	log.Println("【StartOKEXService】")
	apiClient := okex.New(config.Config.OKApiKey, config.Config.OKApiSecret, config.Config.OKPassPhrase)

	placeSellOrderJob := func() {
		log.Println("【placeSellOrderJob】start of job")
		profitRate := 1.018
		placeSellOrders("BTC-JPY", "BTC", profitRate, apiClient)
		placeSellOrders("ETH-JPY", "ETH", profitRate, apiClient)
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

	isTest := false
	if !isTest {
		scheduler.Every(30).Seconds().Run(syncOrderListJob)
		scheduler.Every(300).Seconds().Run(syncSellOrderListJob)
		scheduler.Every(55).Seconds().Run(placeSellOrderJob)
	}
	runtime.Goexit()
}
