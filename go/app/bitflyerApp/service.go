package bitflyerApp

import (
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/bitflyer"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/config"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/enums"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/slack"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/utils"
	"github.com/carlescere/scheduler"
)

var slackClient *slack.APIClient

func StartBfService() {
	log.Println("【StartBfService】start")
	apiClient := bitflyer.NewBitflyer(
		config.Config.ApiKey,
		config.Config.ApiSecret,
		config.Config.BFMaxSell,
		config.Config.BFMaxBuy,
	)

	slackClient = slack.NewSlack(
		config.Config.SlackToken,
		"C01HQKSTK5G",
		"C01M257KX1C",
		config.Config.SlackAPIURL,
	)

	buyingBTCJob := func() {
		placeBuyOrder(enums.Stg0BtcLtp3low7, "BTC_JPY", config.Config.BFBTCBuyAmount01, apiClient, utils.ToP(enums.WeekdayThursday))
	}

	// buyingBTCJob02 := func() {
	// 	placeBuyOrder(enums.Stg1BtcLtp997, "BTC_JPY", config.Config.BFBTCBuyAmount02, apiClient, nil)
	// }

	// buyingBTCJob03 := func() {
	// 	placeBuyOrder(enums.Stg2BtcLtp98, "BTC_JPY", config.Config.BFBTCBuyAmount03, apiClient, nil)
	// }

	// buyingBTCJob99 := func() {
	// 	placeBuyOrder(enums.Stg3BtcLtp90, "BTC_JPY", config.Config.BFBTCBuyAmount03, apiClient, nil)
	// }

	buyingETHJob := func() {
		placeBuyOrder(enums.Stg10EthLtp995, "ETH_JPY", config.Config.BFETHBuyAmount01, apiClient, utils.ToP(enums.WeekdayThursday))
	}

	// buyingETHJob02 := func() {
	// 	placeBuyOrder(enums.Stg11EthLtp98, "ETH_JPY", config.Config.BFETHBuyAmount02, apiClient, nil)
	// }

	// buyingETHJob03 := func() {
	// 	placeBuyOrder(enums.Stg12EthLtp97, "ETH_JPY", config.Config.BFETHBuyAmount03, apiClient, nil)
	// }

	// buyingETHJob04 := func() {
	// 	placeBuyOrder(enums.Stg13EthLtp3low7, "ETH_JPY", config.Config.BFETHBuyAmount03, apiClient, nil)
	// }

	// buyingETHJob99 := func() {
	// 	placeBuyOrder(enums.Stg14EthLtp90, "ETH_JPY", config.Config.BFETHBuyAmount03, apiClient, nil)
	// }

	btcFilledCheckJob := func() {
		filledCheckJob("BTC_JPY", apiClient)
	}

	ethFilledCheckJob := func() {
		filledCheckJob("ETH_JPY", apiClient)
	}

	sellOrderJob := func() {
		placeSellOrder(apiClient)
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
		// 一定期間が経過した買い注文は削除するようにする
		log.Println("【cancelBuyOrderJob】Start of job")
		buyOrders, err := models.GetUnfilledBuyOrders()
		if err != nil {
			log.Printf("## failed to cancel order....")
			goto ENDOFCENCELORDER
		}

		for i, order := range buyOrders {
			log.Printf("## %v %v", i, order.OrderID)
			timestamp, err := time.Parse(utils.Layout, order.Timestamp)
			if err != nil {
				log.Printf("## failed to cancel order....")
				goto ENDOFCENCELORDER
			}
			cancelCriteria := time.Now().AddDate(0, 0, utils.BfCancelCriteria)

			if cancelCriteria.After(timestamp) {
				cancelOrderParam := &bitflyer.Order{
					ProductCode:            order.ProductCode,
					ChildOrderAcceptanceID: order.OrderID,
				}
				apiClient.CancelOrder(cancelOrderParam)
				models.UpdateCancelledBuyOrder(order.OrderID)
				log.Printf("### %v is cancelled!!", order.OrderID)
				slackClient.PostMessage(fmt.Sprintf("Cancelled BuyOrder: OrderId:%v", order.OrderID), true)
			}
		}

	ENDOFCENCELORDER:
		log.Println("【cancelBuyOrderJob】End of job")
	}

	if !config.Config.IsTest {
		scheduler.Every().Day().At("14:53").Run(buyingBTCJob)
		scheduler.Every().Day().At("14:53").Run(buyingETHJob)
		// scheduler.Every().Thursday().At("14:35").Run(buyingBTCJob)
		// scheduler.Every().Thursday().At("14:35").Run(buyingETHJob)

		scheduler.Every().Day().At("23:45").Run(cancelBuyOrderJob)
	} else {
		scheduler.Every(240).Seconds().Run(sellOrderJob)
		scheduler.Every(90).Seconds().Run(syncBTCBuyOrderJob)
		scheduler.Every(90).Seconds().Run(syncETHBuyOrderJob)
		scheduler.Every(90).Seconds().Run(ethFilledCheckJob)
		scheduler.Every(90).Seconds().Run(btcFilledCheckJob)
		scheduler.Every(7200).Seconds().Run(deleteRecordJob)
		// 動作確認用のジョブ
		// scheduler.Every(100000).Seconds().Run(buyingBTCJob)
		// scheduler.Every(100000).Seconds().Run(buyingETHJob)
	}
	runtime.Goexit()
}
