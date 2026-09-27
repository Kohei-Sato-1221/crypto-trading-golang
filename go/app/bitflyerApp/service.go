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
)

// shutdownWaitMinutes は gracefulShutdown が終了する前に静止する時間(分)。
// この間は新規ジョブがすべてブロックされる。trigger_time_09 と合わせて
// 「毎日いつからいつまでボットが止まるか」が決まる。
const shutdownWaitMinutes = 15

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

/*
gracefulShutdown は新規ジョブをブロックし、実行中のジョブの完了を待ってから終了する。

終了後は systemd の Restart=always / RestartSec=10 により約10秒後に再起動される。
これが日次リフレッシュの実体であり、ジョブのハングからの唯一の復旧手段でもある
（scheduler の runJob() は実行中フラグが立ったままのジョブを二度と起動しないため）。

実行中ジョブの完了を待ったあと、終了前に shutdownWaitMinutes ぶんウェイトする。
この間 wrapJob() が新規ジョブをすべてブロックするため、90秒/180秒間隔のジョブ
(syncBuyOrders / filledCheck / sellOrder) も止まる。これは意図した静止時間である。
trigger_time_09=10:30 なら 10:30〜10:45 がその窓にあたり、再開は 10:45 頃になる。

暗号資産市場は24時間動いており、この窓を昼夜どちらに置いても市場条件は変わらない。
日中に置いているのは、異常が起きたとき人が気づいて対処できるようにするためである。
(2026-09-27 に毎晩の電源断をやめて24時間稼働へ移行したのに伴い 01:20 から移動した)
*/
func gracefulShutdown(timeoutMinutes int) {
	log.Println("【app】グレースフルシャットダウン開始 - 新しいジョブの実行をブロックします")

	// シャットダウンフラグを設定して、新しいジョブの実行をブロック。
	// Slack通知より先に行う。通知は最大15秒かかるため、先に投げると
	// その間に新しいジョブが走り出してしまう
	shuttingDown.Lock()
	isShuttingDown = true
	shuttingDown.Unlock()
	log.Println("【app】新しいジョブの実行をブロックしました")

	notifyShutdown(timeoutMinutes)

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

	// 終了前の静止時間。この間は wrapJob() が新規ジョブをブロックし続ける
	log.Printf("【app】%d分間ウェイトして、新しいジョブが発生しないようにします", shutdownWaitMinutes)
	time.Sleep(time.Duration(shutdownWaitMinutes) * time.Minute)
	log.Printf("【app】%d分間のウェイトが完了しました", shutdownWaitMinutes)

	log.Println("【app】アプリケーションを終了します（systemdが約10秒後に再起動します）")
	os.Exit(0)
}

/*
notifyShutdown は日次リフレッシュによる停止をSlackへ通知する。

Slack送信は15秒でタイムアウトする（go/slack のhttpClientTimeout）。ここでハングすると
プロセスが終了できず再起動も起きないため、タイムアウトが設定されていることが前提になる。
*/
func notifyShutdown(timeoutMinutes int) {
	if slackClient == nil {
		return
	}
	now := time.Now()
	msg := fmt.Sprintf(
		"🔄 bfTradingApp を停止します（日次リフレッシュ）\n"+
			"時刻: %s\n"+
			"実行中ジョブの完了を待機（最大%d分）→ %d分ウェイト → 終了\n"+
			"この間すべてのジョブが止まります。systemd により約10秒後に再起動します（再開見込み %s頃）",
		now.Format("2006-01-02 15:04:05 MST"), timeoutMinutes, shutdownWaitMinutes,
		now.Add(time.Duration(shutdownWaitMinutes)*time.Minute).Format("15:04"))
	if err := slackClient.PostMessage(msg, false); err != nil {
		log.Printf("【app】停止通知のSlack送信に失敗しました: %v", err)
	}
}

/*
notifyStartup は起動をSlackへ通知する。

日次リフレッシュ以外の時刻にこの通知が届いたら、クラッシュして自動再起動したことを
意味する。無言のクラッシュに気づくための監視点として機能させる。

注意: systemd の Restart=always / RestartSec=10 と組み合わさるため、万一クラッシュ
ループに陥ると10秒間隔でSlackへ通知が飛ぶ。頻発する場合は unit に
StartLimitIntervalSec / StartLimitBurst を設定して再起動回数を制限すること。
*/
func notifyStartup(r *jobRegistry) {
	if slackClient == nil {
		return
	}
	testLabel := "false"
	if config.Config.IsTest {
		testLabel = "true ⚠️テストモード（本番の買い注文ジョブは登録されません）"
	}
	msg := fmt.Sprintf(
		"🟢 bfTradingApp を起動しました\n"+
			"起動時刻: %s\n"+
			"ジョブ登録: 成功 %d件 / 失敗 %d件\n"+
			"取引所: %s / is_test: %s\n"+
			"次回の自動再起動: %s（gracefulShutdown）",
		time.Now().Format("2006-01-02 15:04:05 MST"),
		r.registered, len(r.failures),
		config.Config.Exchange, testLabel,
		config.Config.TriggerTime09)
	if err := slackClient.PostMessage(msg, false); err != nil {
		log.Printf("【StartBfService】起動通知のSlack送信に失敗しました: %v", err)
	}
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

		// アプリの日次リフレッシュ（trigger_time_09=10:30 JST。10:30〜10:45 が停止窓）。
		// 終了後は systemd の Restart=always により約10秒後に再起動される。
		// かつては Pi が 01:30 JST に電源断される運用に合わせて 01:20 に置いていたが、
		// 2026-09-27 に24時間稼働へ移行したため、状態を確認しやすい日中へ移した。
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

	// 起動したことをSlackへ通知する。想定外の時刻に届けばクラッシュ再起動の合図になる
	notifyStartup(registry)

	runtime.Goexit()
}
