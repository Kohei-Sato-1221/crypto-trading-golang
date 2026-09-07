package bitflyerApp

import (
	"errors"
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

/*
validateBuyPriceSources は発注価格の算出に必要な価格情報が揃っているかを検証する。

以前は `ticker, _ := apiClient.GetTicker(productCode)` と error を握り潰していたため、
取引所APIの一時障害で ticker が nil のまま ticker.Ltp を参照して panic していた。
systemd の Restart=always で再起動すると、起動直後の不正確なシステム時刻から
スケジュールが再計算され意図しない発注につながりうるため、必ず error を確認する。

  - bitflyer の Ticker は BTC/ETH どちらの戦略でもログ出力に使うため常に必須。
  - bitbank の Ticker は BTC_JPY 戦略(strategy < 10)の買値算出にのみ使うため、
    その場合だけ必須とする（bitbank の障害で ETH の発注まで止めないため）。

判定不能なときは「発注しない」側に倒すため、不足があれば error を返す。
戻り値の error はそのまま Slack 通知の本文に載せる。
*/
func validateBuyPriceSources(strategy int, productCode string, ticker *bitflyer.Ticker, tickerErr error,
	bbTicker *bitbank.ReturnTicker, bbErr error) error {

	if tickerErr != nil {
		return fmt.Errorf("bitflyer GetTicker に失敗しました (product_code=%s): %w", productCode, tickerErr)
	}
	if ticker == nil {
		return fmt.Errorf("bitflyer GetTicker がレスポンスを返しませんでした (product_code=%s)", productCode)
	}
	if ticker.Ltp <= 0 || ticker.BestBid <= 0 {
		return fmt.Errorf("bitflyer Ticker の価格が不正です (product_code=%s ltp=%v best_bid=%v)",
			productCode, ticker.Ltp, ticker.BestBid)
	}

	// strategy >= 10 は bitflyer の Ticker だけで買値を算出するため bitbank は不要
	if strategy >= 10 {
		return nil
	}
	if bbErr != nil {
		return fmt.Errorf("bitbank GetBBTicker に失敗しました: %w", bbErr)
	}
	if bbTicker == nil {
		return errors.New("bitbank GetBBTicker がレスポンスを返しませんでした")
	}
	if bbTicker.Last <= 0 || bbTicker.Low <= 0 {
		return fmt.Errorf("bitbank Ticker の価格が不正です (last=%v low=%v)", bbTicker.Last, bbTicker.Low)
	}
	return nil
}

/*
売り注文の上限超過の警告は、解消まで数週間続きうる一方で買い注文ジョブは1日に最大12本走る。
同一内容の警告が件数ぶん飛んで本物の通知が埋もれるのを避けるため、この間隔まで集約する。
通知は消さない（reconcileJob も日次で1通通知する）。
*/
const (
	sellOverLimitNotifyInterval = 24 * time.Hour
	// スロット上限は通貨ペアごとではなく全体で1つなので、キーも1つにする
	sellOverLimitThrottleKey = "sell_over_limit"
)

var sellOverLimitThrottle = newNotificationThrottle(sellOverLimitNotifyInterval)

func placeBuyOrder(strategy int, productCode string, size float64, apiClient *bitflyer.APIClient, weekday *string) {
	weekdayStr := "nil"
	if weekday != nil {
		weekdayStr = *weekday
	}
	slackClient.PostMessage(fmt.Sprintf("placeBuyOrder: code:%s strategry:%v size:%v weekday:%s", productCode, strategy, size, weekdayStr), false)

	log.Printf("strategy:%v", strategy)
	log.Println("【buyingJob】start of job")

	// 今日が指定された曜日でなければスキップする
	if weekday != nil && !enums.IsTodayWeekday(*weekday) {
		msg := fmt.Sprintf("【buyingJob】Skipped!! Today is not %s\n", *weekday)
		log.Println(msg)
		slackClient.PostMessage(msg, true)
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

	// スロット状況を確認する
	// - buy側が上限に達している場合は発注をスキップする
	// - sell側が上限に達している場合はSlack警告のみ出し、発注は続行する
	slotStatus, err := models.GetBuyOrderSlotStatus(apiClient.Max_buy_orders, apiClient.Max_sell_orders)
	if err != nil {
		errMsg := fmt.Sprintf("【ERROR】placeBuyOrder error:%v", err.Error())
		log.Printf("%s\n", errMsg)
		slackClient.PostMessage(errMsg, true)
		log.Println("【buyingJob】end of job as error")
		return
	}
	if slotStatus.ShouldSkip {
		// スキップ理由が他の通知に埋もれないよう絵文字を先頭に置き、件数と上限値の両方を本文に含める
		msg := fmt.Sprintf("🚨【buyingJob】発注スキップ: 未約定の買い注文が上限に達しています %s (%s size:%v strategy:%v)",
			slotStatus.Message, productCode, size, strategy)
		log.Println(msg)
		log.Println("【buyingJob】end of job as skip")
		slackClient.PostMessage(msg, true)
		return
	}
	if slotStatus.SellWarning {
		// 売り注文の滞留では発注をブロックしない（現物積み上がりの歯止めはbudget_criteria）
		warnMsg := fmt.Sprintf("🚨【buyingJob】売り注文が上限超過: %s （発注は続行します）", slotStatus.Message)
		log.Println(warnMsg)
		// 超過状態は解消まで数週間続きうる。買い注文ジョブは1日に最大12本走るため、
		// 毎回通知すると同一内容の 🚨 が1日に何通も流れて本物の通知が埋もれる。
		// 通知を消すのではなく間隔をあけて鳴らす（reconcileJob も日次で1通通知する）
		if sellOverLimitThrottle.shouldNotify(sellOverLimitThrottleKey, time.Now()) {
			slackClient.PostMessage(warnMsg+fmt.Sprintf("\n※同一の警告は%v に1回まで集約しています（解消するまで reconcileJob が日次でも通知します）",
				sellOverLimitNotifyInterval), true)
		}
	}

	// 発注価格の算出に必要な価格情報を取得する。
	// 取得できなかった場合は発注しない（判定不能なときは発注しない側に倒す）
	bitbankClient, bbErr := bitbank.GetBBTicker("btc_jpy")
	ticker, tickerErr := apiClient.GetTicker(productCode)
	if err := validateBuyPriceSources(strategy, productCode, ticker, tickerErr, bitbankClient, bbErr); err != nil {
		errMsg := fmt.Sprintf("🚨【buyingJob】発注価格を算出できないため発注しません: %v (%s size:%v strategy:%v weekday:%s)",
			err, productCode, size, strategy, weekdayStr)
		log.Println(errMsg)
		slackClient.PostMessage(errMsg, true)
		log.Println("【buyingJob】end of job as error")
		return
	}
	log.Printf("bitbankClient  %v", bitbankClient)

	buyPrice := 0.0
	var res *bitflyer.PlaceOrderResponse

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
	// 注文の有効期限(UTC)を記録する。取引所側の厳密な期限は syncBuyOrders がAPIの expire_date で補正する
	expireDate := utc_current_date.Add(time.Duration(minuteToExpire) * time.Minute)
	event := models.OrderEvent{
		OrderID:     res.OrderId,
		Time:        utc_current_date,
		ProductCode: productCode,
		Side:        "BUY",
		Price:       buyPrice,
		Size:        size,
		Exchange:    "bitflyer",
		Strategy:    strategy,
		ExpireDate:  &expireDate,
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

	slackClient.PostMessage(fmt.Sprintf("BuyOrder: %s(%.2f/%v) OrderId:%v expire(UTC):%s",
		productCode, buyPrice, size, res.OrderId, expireDate.Format(time.RFC3339)), true)

	log.Println("【buyingJob】end of job")
}
