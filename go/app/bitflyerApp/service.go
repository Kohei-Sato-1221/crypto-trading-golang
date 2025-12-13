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
	slackClient    *slack.APIClient
	runningJobs    sync.WaitGroup // 実行中のジョブを追跡
	shuttingDown   sync.RWMutex   // シャットダウン中かどうかを保護するRWMutex
	isShuttingDown bool           // シャットダウン中かどうかのフラグ
)

// InitSlackClient Slackクライアントを初期化する
func InitSlackClient() {
	slackClient = slack.NewSlack(
		config.Config.SlackToken,
		"C01HQKSTK5G",
		"C01M257KX1C",
		config.Config.SlackAPIURL,
	)
}

// wrapJob はジョブをラップして、実行開始時にWaitGroupに追加し、終了時にDoneを呼びます
// シャットダウン中は新しいジョブの実行をブロックします
func wrapJob(job func()) func() {
	return func() {
		// シャットダウン中かチェック
		shuttingDown.RLock()
		if isShuttingDown {
			shuttingDown.RUnlock()
			log.Println("【app】シャットダウン中のため、ジョブの実行をスキップします")
			return
		}
		shuttingDown.RUnlock()

		runningJobs.Add(1)
		defer runningJobs.Done()
		job()
	}
}

// gracefulShutdown は実行中のジョブが完了するまで待機し、その後15分間ウェイトしてから終了します
func gracefulShutdown(timeoutMinutes int) {
	log.Println("【app】グレースフルシャットダウン開始 - 新しいジョブの実行をブロックします")

	// シャットダウンフラグを設定して、新しいジョブの実行をブロック
	shuttingDown.Lock()
	isShuttingDown = true
	shuttingDown.Unlock()
	log.Println("【app】新しいジョブの実行をブロックしました")

	log.Println("【app】実行中のジョブの完了を待機します")
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

	// 15分間ウェイトして、新しいジョブが発生しないようにする
	waitMinutes := 15
	log.Printf("【app】%d分間ウェイトして、新しいジョブが発生しないようにします", waitMinutes)
	time.Sleep(time.Duration(waitMinutes) * time.Minute)
	log.Printf("【app】%d分間のウェイトが完了しました", waitMinutes)

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

	buyingBTCJobEveryDay := func() {
		placeBuyOrder(enums.StrategyLTP99, "BTC_JPY", config.Config.BFBTCBuyAmount01, apiClient, nil)
	}
	buyingETHJobEveryDay := func() {
		placeBuyOrder(enums.StrategyLTP99, "ETH_JPY", config.Config.BFETHBuyAmount01, apiClient, nil)
	}

	buyingBTCJobLTP95Mon := func() {
		placeBuyOrder(enums.StrategyLTP95, "BTC_JPY", config.Config.BFBTCBuyAmount01, apiClient, utils.ToP(enums.WeekdayMonday))
	}
	buyingETHJobLTP95Mon := func() {
		placeBuyOrder(enums.StrategyLTP95, "ETH_JPY", config.Config.BFETHBuyAmount01, apiClient, utils.ToP(enums.WeekdayMonday))
	}

	buyingBTCJobLTP98Tue := func() {
		placeBuyOrder(enums.StrategyLTP98, "BTC_JPY", config.Config.BFBTCBuyAmount01, apiClient, utils.ToP(enums.WeekdayTuesday))
	}
	buyingETHJobLTP98Tue := func() {
		placeBuyOrder(enums.StrategyLTP98, "ETH_JPY", config.Config.BFETHBuyAmount01, apiClient, utils.ToP(enums.WeekdayTuesday))
	}

	buyingBTCJobLTP5t5Wed := func() {
		placeBuyOrder(enums.StrategyLtpLowestIn7days5t5, "BTC_JPY", config.Config.BFBTCBuyAmount01, apiClient, utils.ToP(enums.WeekdayWednesday))
	}
	buyingETHJobLTP5t5Wed := func() {
		placeBuyOrder(enums.StrategyLtpLowestIn7days5t5, "ETH_JPY", config.Config.BFETHBuyAmount01, apiClient, utils.ToP(enums.WeekdayWednesday))
	}

	buyingBTCJobLTP98Sat := func() {
		placeBuyOrder(enums.StrategyLTP98, "BTC_JPY", config.Config.BFBTCBuyAmount01, apiClient, utils.ToP(enums.WeekdaySaturday))
	}
	buyingETHJobLTP98Sat := func() {
		placeBuyOrder(enums.StrategyLTP98, "ETH_JPY", config.Config.BFETHBuyAmount01, apiClient, utils.ToP(enums.WeekdaySaturday))
	}

	buyingBTCJobLTP5t5Sun := func() {
		placeBuyOrder(enums.StrategyLtpLowestIn7days2t8, "BTC_JPY", config.Config.BFBTCBuyAmount01, apiClient, utils.ToP(enums.WeekdaySunday))
	}
	buyingETHJobLTP5t5Sun := func() {
		placeBuyOrder(enums.StrategyLtpLowestIn7days2t8, "ETH_JPY", config.Config.BFETHBuyAmount01, apiClient, utils.ToP(enums.WeekdaySunday))
	}

	buyingBTCJobLTP95TEST := func() {
		placeBuyOrder(enums.StrategyLTP95, "BTC_JPY", config.Config.BFBTCBuyAmount01, apiClient, utils.ToP(enums.WeekdaySaturday))
	}
	buyingETHJobLTP95TEST := func() {
		placeBuyOrder(enums.StrategyLTP95, "ETH_JPY", config.Config.BFETHBuyAmount01, apiClient, utils.ToP(enums.WeekdaySaturday))
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
		SavePriceHistoryJob(apiClient)
	}

	sendResultsJobFunc := func() {
		SendResultsJob(apiClient)
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
		scheduler.Every().Day().At("17:30").Run(wrapJob(buyingBTCJobEveryDay))
		scheduler.Every().Day().At("17:30").Run(wrapJob(buyingETHJobEveryDay))

		scheduler.Every().Day().At("17:30").Run(wrapJob(buyingBTCJobLTP95Mon))
		scheduler.Every().Day().At("17:30").Run(wrapJob(buyingETHJobLTP95Mon))

		scheduler.Every().Day().At("17:30").Run(wrapJob(buyingBTCJobLTP98Tue))
		scheduler.Every().Day().At("17:30").Run(wrapJob(buyingETHJobLTP98Tue))

		scheduler.Every().Day().At("17:30").Run(wrapJob(buyingBTCJobLTP5t5Wed))
		scheduler.Every().Day().At("17:30").Run(wrapJob(buyingETHJobLTP5t5Wed))

		scheduler.Every().Day().At("17:30").Run(wrapJob(buyingBTCJobLTP98Sat))
		scheduler.Every().Day().At("17:30").Run(wrapJob(buyingETHJobLTP98Sat))

		scheduler.Every().Day().At("17:30").Run(wrapJob(buyingBTCJobLTP5t5Sun))
		scheduler.Every().Day().At("17:30").Run(wrapJob(buyingETHJobLTP5t5Sun))

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
		scheduler.Every().Day().At("06:45").Run(wrapJob(sendResultsJobFunc))

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
		// scheduler.Every().Day().At("15:35").Run(wrapJob(savePriceHistoryJobFunc))
		scheduler.Every().Day().At("16:24").Run(wrapJob(buyingBTCJobLTP95TEST))
		scheduler.Every().Day().At("16:24").Run(wrapJob(buyingETHJobLTP95TEST))
	}
	runtime.Goexit()
}
