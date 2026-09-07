package bitflyerApp

import (
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
		config.Config.BFMaxBuy,
		config.Config.BFMaxSell,
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

	buyingBTCJobLTP97Mon := func() {
		placeBuyOrder(enums.StrategyLTP97, "BTC_JPY", config.Config.BFBTCBuyAmount01, apiClient, utils.ToP(enums.WeekdayMonday))
	}
	buyingETHJobLTP97Mon := func() {
		placeBuyOrder(enums.StrategyLTP97, "ETH_JPY", config.Config.BFETHBuyAmount01, apiClient, utils.ToP(enums.WeekdayMonday))
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

	buyingBTCJobLTP7t3Sun := func() {
		placeBuyOrder(enums.StrategyLtpLowestIn7days7t3, "BTC_JPY", config.Config.BFBTCBuyAmount01, apiClient, utils.ToP(enums.WeekdaySunday))
	}
	buyingETHJobLTP7t3Sun := func() {
		placeBuyOrder(enums.StrategyLtpLowestIn7days7t3, "ETH_JPY", config.Config.BFETHBuyAmount01, apiClient, utils.ToP(enums.WeekdaySunday))
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

	// 期限が近い売り注文をキャンセル→同条件で再発注して実質無期限化する（実装は rolloverSellOrderJob.go）
	rolloverSellOrderJobFunc := func() {
		rolloverSellOrderJob(apiClient)
	}

	// 失効した注文をCANCELLEDに落としてスロットを解放する（実装は expireSweepJob.go）
	expireSweepJobFunc := func() {
		expireSweepJob(apiClient)
	}

	// 長期間約定しない買い注文を能動キャンセルする（実装は cancelBuyOrderJob.go）
	cancelBuyOrderJobFunc := func() {
		cancelBuyOrderJob(apiClient)
	}

	// 取引所とDBを突合し、乖離・発注ゼロ・スロット逼迫をSlackへ通知する（実装は reconcileJob.go）
	reconcileJobFunc := func() {
		reconcileJob(apiClient)
	}

	triggerTime01 := config.Config.TriggerTime01
	triggerTime02 := config.Config.TriggerTime02
	triggerTime03 := config.Config.TriggerTime03
	triggerTime04 := config.Config.TriggerTime04
	triggerTime05 := config.Config.TriggerTime05
	triggerTime06 := config.Config.TriggerTime06
	triggerTime07 := config.Config.TriggerTime07
	triggerTime08 := config.Config.TriggerTime08
	triggerTime09 := config.Config.TriggerTime09

	// ジョブ登録はすべて registry 経由で行う。
	// scheduler.Run() の戻り値を検査せずに登録すると、登録に失敗したジョブが
	// 「ログにもSlackにも出ないまま二度と発火しない」という無言のデグレになるため（F30-2）。
	// 失敗してもプロセスは落とさず、起動時に1通だけSlack通知する（理由は jobRegistry.go を参照）。
	registry := &jobRegistry{}

	if !config.Config.IsTest {
		registry.daily("buyingBTCJobEveryDay", triggerTime01, buyingBTCJobEveryDay)
		registry.daily("buyingETHJobEveryDay", triggerTime01, buyingETHJobEveryDay)

		registry.daily("buyingBTCJobLTP97Mon", triggerTime01, buyingBTCJobLTP97Mon)
		registry.daily("buyingETHJobLTP97Mon", triggerTime01, buyingETHJobLTP97Mon)

		registry.daily("buyingBTCJobLTP98Tue", triggerTime01, buyingBTCJobLTP98Tue)
		registry.daily("buyingETHJobLTP98Tue", triggerTime01, buyingETHJobLTP98Tue)

		registry.daily("buyingBTCJobLTP5t5Wed", triggerTime01, buyingBTCJobLTP5t5Wed)
		registry.daily("buyingETHJobLTP5t5Wed", triggerTime01, buyingETHJobLTP5t5Wed)

		registry.daily("buyingBTCJobLTP98Sat", triggerTime01, buyingBTCJobLTP98Sat)
		registry.daily("buyingETHJobLTP98Sat", triggerTime01, buyingETHJobLTP98Sat)

		registry.daily("buyingBTCJobLTP7t3Sun", triggerTime01, buyingBTCJobLTP7t3Sun)
		registry.daily("buyingETHJobLTP7t3Sun", triggerTime01, buyingETHJobLTP7t3Sun)

		registry.interval("syncBTCBuyOrderJob", 90, syncBTCBuyOrderJob)
		registry.interval("syncETHBuyOrderJob", 90, syncETHBuyOrderJob)
		registry.interval("sellOrderJob", 180, sellOrderJob)
		registry.interval("ethFilledCheckJob", 90, ethFilledCheckJob)
		registry.interval("btcFilledCheckJob", 90, btcFilledCheckJob)
		registry.interval("deleteRecordJob", 7200, deleteRecordJob)

		// 毎日6時と18時に価格履歴を保存
		registry.daily("savePriceHistoryJob(夕)", triggerTime03, savePriceHistoryJobFunc)
		registry.daily("savePriceHistoryJob(朝)", triggerTime04, savePriceHistoryJobFunc)

		// 毎日朝9時に収益結果をSlackに送信
		registry.daily("sendResultsJob", triggerTime02, sendResultsJobFunc)

		// 売り注文の27日ローリング（05:30 JST）。1日1回。
		// 失効検出(06:05)より前に走らせ、巻き直せた注文がsweepの対象にならないようにする。
		// 実行環境はRaspberry Pi上のsystemdサービスで原則24時間稼働。ただしPi自体が毎日01:30〜02:45 JSTに停止するため、その時間帯を避けている
		registry.daily("rolloverSellOrderJob", triggerTime05, rolloverSellOrderJobFunc)

		// 失効検出（06:05 JST）。買い注文ジョブ(trigger_time_01=06:30)より前に走らせ、
		// スロットのカウントが正しい状態で発注判定させる
		registry.daily("expireSweepJob", triggerTime06, expireSweepJobFunc)

		// 日次リコンサイル（06:15 JST）。1日1回。
		// 失効検出(06:05)の後・買い注文ジョブ(trigger_time_01=06:30)の前に走らせ、
		// スロット解放が済んだ状態のDBと取引所を突合する。
		// 実行環境のRaspberry Piは毎日 01:30〜02:45 JST に停止するため、その時間帯は避けている
		registry.daily("reconcileJob", triggerTime07, reconcileJobFunc)

		// 買い注文の能動キャンセル（trigger_time_08=22:45 JST）。
		// 旧設定は23:45。移動時は「EC2稼働窓の外で発火しない」という前提だったが、実行環境はRaspberry Piの24時間稼働で
		// 23:45でも発火していた。22:45もPi停止時間帯(01:30〜02:45 JST)を避けており支障がないため据え置いている
		registry.daily("cancelBuyOrderJob", triggerTime08, cancelBuyOrderJobFunc)

		// アプリをグレースフルシャットダウン（trigger_time_09=01:20 JST。実行中のジョブ完了を待機）。
		// Pi停止(01:30 JST)の直前に置いている。
		// 自分自身が runningJobs の完了を待つため、wrapJob() は通さない（dailyWithoutWrap）
		registry.dailyWithoutWrap("gracefulShutdown", triggerTime09, func() {
			gracefulShutdown(5) // 最大5分待機
		})
	} else {
		// 動作確認用のジョブ
		registry.daily("buyingBTCJobLTP95TEST", "16:24", buyingBTCJobLTP95TEST)
		registry.daily("buyingETHJobLTP95TEST", "16:24", buyingETHJobLTP95TEST)
	}

	// 登録結果をログに残し、失敗があれば起動時に1通Slack通知する
	registry.reportRegistrationResult()

	runtime.Goexit()
}
