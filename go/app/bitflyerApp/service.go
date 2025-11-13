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
		placeBuyOrder(enums.Stg0BtcLtp3low7, "BTC_JPY", config.Config.BFBTCBuyAmount01, apiClient)
	}

	// buyingBTCJob02 := func() {
	// 	placeBuyOrder(enums.Stg1BtcLtp997, "BTC_JPY", config.Config.BFBTCBuyAmount02, apiClient)
	// }

	// buyingBTCJob03 := func() {
	// 	placeBuyOrder(enums.Stg2BtcLtp98, "BTC_JPY", config.Config.BFBTCBuyAmount03, apiClient)
	// }

	// buyingBTCJob99 := func() {
	// 	placeBuyOrder(enums.Stg3BtcLtp90, "BTC_JPY", config.Config.BFBTCBuyAmount03, apiClient)
	// }

	buyingETHJob := func() {
		placeBuyOrder(enums.Stg10EthLtp995, "ETH_JPY", config.Config.BFETHBuyAmount01, apiClient)
	}

	// buyingETHJob02 := func() {
	// 	placeBuyOrder(enums.Stg11EthLtp98, "ETH_JPY", config.Config.BFETHBuyAmount02, apiClient)
	// }

	// buyingETHJob03 := func() {
	// 	placeBuyOrder(enums.Stg12EthLtp97, "ETH_JPY", config.Config.BFETHBuyAmount03, apiClient)
	// }

	// buyingETHJob04 := func() {
	// 	placeBuyOrder(enums.Stg13EthLtp3low7, "ETH_JPY", config.Config.BFETHBuyAmount03, apiClient)
	// }

	// buyingETHJob99 := func() {
	// 	placeBuyOrder(enums.Stg14EthLtp90, "ETH_JPY", config.Config.BFETHBuyAmount03, apiClient)
	// }

	btcFilledCheckJob := func() {
		filledCheckJob("BTC_JPY", apiClient)
	}

	ethFilledCheckJob := func() {
		filledCheckJob("ETH_JPY", apiClient)
	}

	sellOrderJob := func() {
		log.Println("【sellOrderjob】start of job")
		// get list of orderis whose filled param equqls "1"
		buyOrderInfos := models.CheckFilledBuyOrders()
		if buyOrderInfos == nil {
			log.Println("【sellOrderjob】 : No order ids ")
			goto ENDOFSELLORDER
		}

		for i, buyOrderInfo := range buyOrderInfos {
			orderID := buyOrderInfo.OrderID
			productCode := buyOrderInfo.ProductCode
			size := buyOrderInfo.Size
			sellPrice := buyOrderInfo.CalculateSellOrderPrice()
			log.Printf("No%d Id:%v sellPrice:%10.2f strategy:%v", i, orderID, sellPrice, buyOrderInfo.Strategy)

			sellOrder := &bitflyer.Order{
				ProductCode:     productCode,
				ChildOrderType:  "LIMIT",
				Side:            "SELL",
				Price:           sellPrice,
				Size:            size,
				MinuteToExpires: 43200, // 30days
				TimeInForce:     "GTC",
			}

			log.Printf("sell order:%v\n", sellOrder)
			res, err := apiClient.PlaceOrder(sellOrder)
			log.Printf("sell res:%v\n", res)
			if err != nil {
				log.Println("SellOrder failed.... Failure in [apiClient.PlaceOrder()]")
				break
			}
			if res.OrderId == "" {
				log.Println("SellOrder failed.... no response")
				break
			}

			err = models.UpdateFilledOrderWithBuyOrder(orderID)
			if err != nil {
				log.Println("Failure to update records..... / #UpdateFilledOrderWithBuyOrder")
				break
			}
			log.Printf("Buy Order updated successfully!! #UpdateFilledOrderWithBuyOrder  orderId:%s", orderID)

			utc, _ := time.LoadLocation("UTC")
			utcCurrentDate := time.Now().In(utc)
			event := models.OrderEvent{
				OrderID:     res.OrderId,
				Time:        utcCurrentDate,
				ProductCode: productCode,
				Side:        "Sell",
				Price:       sellPrice,
				Size:        size,
				Exchange:    "bitflyer",
			}
			err = event.SellOrder(orderID)
			if err != nil {
				log.Println("BuyOrder failed.... Failure in [event.BuyOrder()]")
			} else {
				log.Printf("BuyOrder Succeeded! OrderId:%v", res.OrderId)
			}
		}
	ENDOFSELLORDER:
		log.Println("【sellOrderjob】end of job")
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
		scheduler.Every(150).Seconds().Run(sellOrderJob)

		scheduler.Every(120).Seconds().Run(syncBTCBuyOrderJob)
		scheduler.Every(120).Seconds().Run(syncETHBuyOrderJob)

		// 木曜日 12:30 JST (日本時間で実行)
		// システムのタイムゾーンがUTCの場合、JST 12:30 = UTC 03:30
		// システムのタイムゾーンがJSTの場合、そのまま "12:30" でOK
		// 確実にJSTで実行するため、UTC時刻に変換して指定
		// JST 12:30 = UTC 03:30 (JSTはUTC+9時間)
		scheduler.Every().Thursday().At("03:45").Run(buyingBTCJob) // JST 12:45
		scheduler.Every().Thursday().At("03:45").Run(buyingETHJob) // JST 12:45

		// scheduler.Every().Tuesday().At("06:55").Run(buyingBTCJob02) // 火曜日
		// scheduler.Every().Day().At("10:55").Run(buyingBTCJob03)
		// scheduler.Every().Day().At("00:53").Run(buyingBTCJob99)

		// scheduler.Every().Day().At("04:53").Run(buyingETHJob02)
		// scheduler.Every().Day().At("07:53").Run(buyingETHJob03)
		// scheduler.Every().Day().At("10:53").Run(buyingETHJob04)
		// scheduler.Every().Day().At("00:55").Run(buyingETHJob99)

		scheduler.Every(45).Seconds().Run(ethFilledCheckJob)
		scheduler.Every(45).Seconds().Run(btcFilledCheckJob)
		scheduler.Every(7200).Seconds().Run(deleteRecordJob)

		scheduler.Every().Day().At("23:45").Run(cancelBuyOrderJob)
	} else {
		// 動作確認用のジョブ
		// scheduler.Every(100000).Seconds().Run(buyingBTCJob)
		// scheduler.Every(100000).Seconds().Run(buyingETHJob)
	}
	runtime.Goexit()
}
