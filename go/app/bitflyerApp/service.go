package bitflyerApp

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/bitflyer"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/config"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/enums"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/slack"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/utils"
	"github.com/carlescere/scheduler"
)

var (
	slackClient *slack.APIClient
	runningJobs sync.WaitGroup // 実行中のジョブを追跡
)

// wrapJob はジョブをラップして、実行開始時にWaitGroupに追加し、終了時にDoneを呼びます
func wrapJob(job func()) func() {
	return func() {
		runningJobs.Add(1)
		defer runningJobs.Done()
		job()
	}
}

// gracefulShutdown は実行中のジョブが完了するまで待機してから終了します
func gracefulShutdown(timeoutMinutes int) {
	log.Println("【app】グレースフルシャットダウン開始 - 実行中のジョブの完了を待機します")

	// タイムアウト付きで待機
	done := make(chan struct{})
	go func() {
		runningJobs.Wait()
		close(done)
	}()

	timeout := time.Duration(timeoutMinutes) * time.Minute
	select {
	case <-done:
		log.Println("【app】すべてのジョブが完了しました")
	case <-time.After(timeout):
		log.Printf("【app】タイムアウト（%d分）経過 - 強制終了します", timeoutMinutes)
	}

	log.Println("【app】アプリケーションを終了します")
	os.Exit(0)
}

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
		placeBuyOrder(enums.Stg0BtcLtp3low7, "BTC_JPY", config.Config.BFBTCBuyAmount01, apiClient, utils.ToP(enums.WeekdayMonday))
	}
	buyingETHJob := func() {
		placeBuyOrder(enums.Stg10EthLtp995, "ETH_JPY", config.Config.BFETHBuyAmount01, apiClient, utils.ToP(enums.WeekdayMonday))
	}

	buyingBTCJob02 := func() {
		placeBuyOrder(enums.Stg1BtcLtp997, "BTC_JPY", config.Config.BFBTCBuyAmount02, apiClient, utils.ToP(enums.WeekdayTuesday))
	}
	buyingETHJob02 := func() {
		placeBuyOrder(enums.Stg11EthLtp98, "ETH_JPY", config.Config.BFETHBuyAmount02, apiClient, utils.ToP(enums.WeekdayTuesday))
	}

	buyingBTCJob03 := func() {
		placeBuyOrder(enums.Stg2BtcLtp98, "BTC_JPY", config.Config.BFBTCBuyAmount03, apiClient, utils.ToP(enums.WeekdayFriday))
	}
	buyingETHJob03 := func() {
		placeBuyOrder(enums.Stg12EthLtp97, "ETH_JPY", config.Config.BFETHBuyAmount03, apiClient, utils.ToP(enums.WeekdayFriday))
	}

	buyingETHJob04 := func() {
		placeBuyOrder(enums.Stg13EthLtp3low7, "ETH_JPY", config.Config.BFETHBuyAmount03, apiClient, utils.ToP(enums.WeekdaySaturday))
	}

	buyingETHJob99 := func() {
		placeBuyOrder(enums.Stg14EthLtp90, "ETH_JPY", config.Config.BFETHBuyAmount03, apiClient, utils.ToP(enums.WeekdaySunday))
	}
	buyingBTCJob99 := func() {
		placeBuyOrder(enums.Stg3BtcLtp90, "BTC_JPY", config.Config.BFBTCBuyAmount03, apiClient, utils.ToP(enums.WeekdaySunday))
	}

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

	savePriceHistoryJobFunc := func() {
		savePriceHistoryJob(apiClient)
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
		scheduler.Every().Day().At("6:30").Run(wrapJob(buyingBTCJob))
		scheduler.Every().Day().At("6:30").Run(wrapJob(buyingETHJob))

		scheduler.Every().Day().At("6:30").Run(wrapJob(buyingBTCJob02))
		scheduler.Every().Day().At("6:30").Run(wrapJob(buyingETHJob02))

		scheduler.Every().Day().At("6:30").Run(wrapJob(buyingBTCJob03))
		scheduler.Every().Day().At("6:30").Run(wrapJob(buyingETHJob03))

		scheduler.Every().Day().At("6:30").Run(wrapJob(buyingETHJob04))

		scheduler.Every().Day().At("6:30").Run(wrapJob(buyingETHJob99))
		scheduler.Every().Day().At("6:30").Run(wrapJob(buyingBTCJob99))

		scheduler.Every(90).Seconds().Run(wrapJob(syncBTCBuyOrderJob))
		scheduler.Every(90).Seconds().Run(wrapJob(syncETHBuyOrderJob))
		scheduler.Every(180).Seconds().Run(wrapJob(sellOrderJob))
		scheduler.Every(90).Seconds().Run(wrapJob(ethFilledCheckJob))
		scheduler.Every(90).Seconds().Run(wrapJob(btcFilledCheckJob))
		scheduler.Every(7200).Seconds().Run(wrapJob(deleteRecordJob))

		// 毎日6時と18時に価格履歴を保存
		scheduler.Every().Day().At("06:00").Run(wrapJob(savePriceHistoryJobFunc))
		scheduler.Every().Day().At("18:00").Run(wrapJob(savePriceHistoryJobFunc))

		// 毎日朝9時に収益結果をSlackに送信
		scheduler.Every().Day().At("06:45").Run(wrapJob(sendResultsJob))
		scheduler.Every().Day().At("23:45").Run(wrapJob(cancelBuyOrderJob))

		// 12:20と22:50にアプリをグレースフルシャットダウン（実行中のジョブ完了を待機）
		scheduler.Every().Day().At("12:20").Run(func() {
			gracefulShutdown(5) // 最大5分待機
		})
		scheduler.Every().Day().At("22:50").Run(func() {
			gracefulShutdown(5) // 最大5分待機
		})
	} else {
		// 動作確認用のジョブ
		// scheduler.Every(100000).Seconds().Run(buyingBTCJob)
		// scheduler.Every(100000).Seconds().Run(buyingETHJob)
	}
	runtime.Goexit()
}
