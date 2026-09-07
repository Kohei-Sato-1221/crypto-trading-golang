package bitflyerApp

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/bitflyer"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/config"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

/*
expireSweepJob は取引所側で失効した注文をDB上で CANCELLED に落とすジョブ。

背景:
Bitflyerは失効した注文をAPIから完全に削除する（child_order_state=EXPIRED は常に0件、
child_order_acceptance_id での個別照会でも取得できない）。そのため「失効した」ことを
取引所APIから直接検知することはできず、DB側にレコードが UNFILLED のまま残り続けて
スロット(max_buy_orders / max_sell_orders)を食い潰す幽霊レコードが発生する。

検出方式:
  - 方式A（主）: DBに保持した expire_date が経過（猶予つき）した UNFILLED レコードを失効とみなす。
  - 方式B（補助）: expire_date が未設定（カラム追加前に取り込まれた旧レコード）については、
    取引所のACTIVE一覧にもCOMPLETED一覧にも存在しないことをもって失効とみなす。

安全側の設計:
  - CANCELLED にする前に必ずCOMPLETED一覧と突合し、約定済みなら FILLED に更新して対象から外す
    （約定した注文を失効と誤判定すると、保有した現物がDB上追跡不能になるため）。
  - 一覧の取得に失敗した通貨ペアは sweep 自体をスキップする
    （GetChildOrdersAll はページング途中でエラーが起きると取得済みページも破棄するため、
    不完全な一覧で「一覧に無い＝失効」と判定すると実在する注文をCANCELLEDにしてしまう）。
  - 方式Bでは、COMPLETED一覧で遡れた最古の child_order_date より古いレコードを判定保留にし、
    件数と order_id をSlackへ通知する（遡り不足による誤判定を防ぐフェイルセーフ）。
  - remarks に models.RemarkRolloverPending を含むレコードは、ローリングがまだ再試行しうる間だけ対象外
    （売り注文のローリング再試行待ちと競合させない）。ローリングの窓を過ぎたレコードは
    どのジョブにも拾われず恒久的に滞留してしまうため、除外を解いて本ジョブがCANCELLEDにする。
  - 売り注文が失効した場合も親の買い注文のステータスは戻さない（自動再発注は行わない）。
    現物は保有されたままになるため、Slack通知のうえユーザーが手動で対応する。
*/

// expireSweepProductCodes は失効検出の対象通貨ペア。
var expireSweepProductCodes = []string{"BTC_JPY", "ETH_JPY"}

const (
	// expireSweepMaxRecords は1テーブル・1通貨ペアあたり1回の実行で処理する上限件数。
	// 想定外の件数を一度にCANCELLEDへ落とさないための安全弁。
	expireSweepMaxRecords = 200

	// expireSweepPendingSampleSize は判定保留のSlack通知に載せる order_id の最大件数。
	expireSweepPendingSampleSize = 10
)

/*
exchangeOrderIndex は取引所APIから取得した注文一覧の索引。

oldestCompleted は COMPLETED 一覧で遡れた最古の child_order_date(UTC)。
これより古いレコードは「一覧に無い＝失効」と断定できないため、方式Bでは判定保留にする。
*/
type exchangeOrderIndex struct {
	activeIDs       map[string]bool
	completedIDs    map[string]bool
	oldestCompleted time.Time
}

// expireSweepSummary はジョブ全体の処理結果。
type expireSweepSummary struct {
	cancelledBuy  int
	cancelledSell int
	filled        int
	pending       int
	pendingIDs    []string
	skippedActive int
	errors        int
}

/*
expireSweepRolloverRetryAfter は「ローリングがまだ再試行しうる」timestamp の下限(UTC)を返す。

expire_date が NULL の旧レコードに対する models.GetSellOrdersToRollover の抽出下限
（timestamp > now - (fallbackDays + daysBefore) 日）と同じ値を使い、
sweep(方式B)が再発注待ちレコードを横取りしないようにする。
この境界より古いレコードはローリングが二度と対象にしないため、sweep の担当に移す。
*/
func expireSweepRolloverRetryAfter(now time.Time) time.Time {
	return now.UTC().AddDate(0, 0, -(rolloverFallbackDays() + rolloverDaysBeforeExpire()))
}

// expireSweepGrace は失効とみなすまでの猶予時間を返す。
func expireSweepGrace() time.Duration {
	minutes := config.Config.BFExpireSweepGraceMinutes
	if minutes <= 0 {
		minutes = config.DefaultExpireSweepGraceMinutes
	}
	return time.Duration(minutes) * time.Minute
}

/*
buildExchangeOrderIndex は指定通貨ペアのACTIVE / COMPLETED一覧を取得して索引を作る。

どちらか一方でも取得に失敗した場合はエラーを返し、呼び出し側はその通貨ペアの
sweep をスキップする（不完全な一覧での誤判定を避けるフェイルセーフ）。
*/
func buildExchangeOrderIndex(apiClient *bitflyer.APIClient, productCode string) (*exchangeOrderIndex, error) {
	completedOrders, oldestCompleted, err := apiClient.GetChildOrdersAll(productCode, "COMPLETED")
	if err != nil {
		return nil, fmt.Errorf("failed to get COMPLETED child orders: %w", err)
	}
	activeOrders, _, err := apiClient.GetChildOrdersAll(productCode, "ACTIVE")
	if err != nil {
		return nil, fmt.Errorf("failed to get ACTIVE child orders: %w", err)
	}

	index := &exchangeOrderIndex{
		activeIDs:       make(map[string]bool, len(activeOrders)),
		completedIDs:    make(map[string]bool, len(completedOrders)),
		oldestCompleted: oldestCompleted,
	}
	for _, order := range activeOrders {
		index.activeIDs[order.ChildOrderAcceptanceID] = true
	}
	for _, order := range completedOrders {
		index.completedIDs[order.ChildOrderAcceptanceID] = true
	}
	log.Printf("【expireSweep】product_code:%s active:%d completed:%d oldestCompletedChildOrderDate(UTC):%s",
		productCode, len(index.activeIDs), len(index.completedIDs), formatSweepTime(index.oldestCompleted))
	return index, nil
}

// formatSweepTime はログ・通知用に時刻を文字列化する。ゼロ値は "なし"。
func formatSweepTime(t time.Time) string {
	if t.IsZero() {
		return "なし"
	}
	return t.UTC().Format(time.RFC3339)
}

// markSweptOrderFilled は約定済みと判明したレコードを FILLED に更新する。
func markSweptOrderFilled(record models.OrderRecord, summary *expireSweepSummary) {
	if err := models.UpdateFilledOrder(record.OrderID); err != nil {
		summary.errors++
		msg := fmt.Sprintf("🚨【expireSweep】約定判明レコードのFILLED更新に失敗: table=%s OrderID=%s %s price=%.2f size=%v err=%v",
			record.Table, record.OrderID, record.ProductCode, record.Price, record.Size, err)
		log.Println(msg)
		slackClient.PostMessage(msg, true)
		return
	}
	summary.filled++
	log.Printf("【expireSweep】約定判明のためFILLEDに更新: table=%s OrderID=%s %s price=%.2f size=%v",
		record.Table, record.OrderID, record.ProductCode, record.Price, record.Size)
}

/*
cancelSweptOrder はレコードを CANCELLED に更新し、集計とSlack通知を行う。

売り注文が失効した場合は、親の買い注文を FILLED に戻さず（＝自動再発注はせず）
「現物が保有されたまま売り注文が存在しない」状態になったことをエラー通知する。
*/
func cancelSweptOrder(record models.OrderRecord, method string, now time.Time, summary *expireSweepSummary) {
	remark := fmt.Sprintf(" / expired at %s (auto sweep, method=%s)", now.UTC().Format(time.RFC3339), method)
	if err := models.MarkOrderCancelledWithRemark(record.Table, record.OrderID, remark); err != nil {
		summary.errors++
		msg := fmt.Sprintf("🚨【expireSweep】CANCELLED更新に失敗: table=%s OrderID=%s %s price=%.2f size=%v method=%s err=%v",
			record.Table, record.OrderID, record.ProductCode, record.Price, record.Size, method, err)
		log.Println(msg)
		slackClient.PostMessage(msg, true)
		return
	}

	log.Printf("【expireSweep】失効を検出しCANCELLEDに更新: table=%s OrderID=%s %s price=%.2f size=%v expire_date(UTC):%s method=%s",
		record.Table, record.OrderID, record.ProductCode, record.Price, record.Size,
		formatSweepExpireDate(record.ExpireDate), method)

	if record.Table == models.TableSellOrders {
		summary.cancelledSell++
		// ローリングの再発注に失敗し続けた末の失効は、原因が分かるよう本文に明示する
		cause := ""
		if strings.Contains(record.Remarks, models.RemarkRolloverPending) {
			cause = fmt.Sprintf("（%s 付き: ローリングの再発注に失敗したまま期限を過ぎました）", models.RemarkRolloverPending)
		}
		msg := fmt.Sprintf("🚨🚨【expireSweep】売り注文が失効しました: OrderID=%s ParentID=%s %s price=%.2f size=%v%s。"+
			"現物は保有されたままです。親買い注文は %s のまま維持します。手動での対応をお願いします",
			record.OrderID, record.ParentID, record.ProductCode, record.Price, record.Size, cause,
			models.OrderStatusFilledSellOrderPlaced)
		log.Println(msg)
		slackClient.PostMessage(msg, true)
		return
	}
	summary.cancelledBuy++
}

// formatSweepExpireDate はログ用に expire_date(UTC) を文字列化する。未設定は "nil"。
func formatSweepExpireDate(expireDate *time.Time) string {
	if expireDate == nil {
		return "nil"
	}
	return expireDate.UTC().Format(time.RFC3339)
}

/*
sweepExpiredOrders は方式A（expire_date の経過）で失効レコードを処理する。

CANCELLED に更新する前に必ずCOMPLETED一覧と突合し、約定済みなら FILLED に更新して対象から外す。
取引所側でまだACTIVEな注文はDBの期限判定より実データを優先し、状態を変えない。
*/
func sweepExpiredOrders(table models.OrderTable, now time.Time, grace time.Duration,
	indexes map[string]*exchangeOrderIndex, summary *expireSweepSummary) {

	records, err := models.GetExpiredUnfilledOrders(table, now, grace, expireSweepMaxRecords)
	if err != nil {
		summary.errors++
		msg := fmt.Sprintf("🚨【expireSweep】期限切れレコードの取得に失敗: table=%s err=%v", table, err)
		log.Println(msg)
		slackClient.PostMessage(msg, true)
		return
	}
	log.Printf("【expireSweep】方式A table:%s 対象候補:%d件 grace:%v", table, len(records), grace)

	for _, record := range records {
		index, ok := indexes[record.ProductCode]
		if !ok {
			// 一覧を取得できなかった通貨ペアは約定済みかどうかを判定できないため触らない
			log.Printf("【expireSweep】一覧未取得のためスキップ: table=%s OrderID=%s product_code=%s",
				table, record.OrderID, record.ProductCode)
			continue
		}
		if index.completedIDs[record.OrderID] {
			markSweptOrderFilled(record, summary)
			continue
		}
		if index.activeIDs[record.OrderID] {
			// 取引所側でまだ生存している注文はDBの期限判定より実データを優先する
			summary.skippedActive++
			log.Printf("【expireSweep】取引所側でACTIVEのためCANCELLEDにしない: table=%s OrderID=%s %s expire_date(UTC):%s",
				table, record.OrderID, record.ProductCode, formatSweepExpireDate(record.ExpireDate))
			continue
		}
		cancelSweptOrder(record, "A", now, summary)
	}
}

/*
sweepOrdersWithoutExpireDate は方式B（一覧からの消滅）で失効レコードを処理する。

expire_date が未設定の旧レコードのみを対象とし、ACTIVE / COMPLETED のどちらの一覧にも
存在しない場合に失効とみなす。ただしCOMPLETED一覧で遡れた最古の child_order_date より
古いレコードは、遡り範囲外にある約定済み注文を誤って失効と判定しうるため判定保留にする。

models.RemarkRolloverPending 付きのレコードは、ローリングがまだ再試行しうる間
（timestamp が rolloverRetryAfter より新しい間）だけ抽出対象から外れる。
*/
func sweepOrdersWithoutExpireDate(table models.OrderTable, now time.Time,
	indexes map[string]*exchangeOrderIndex, summary *expireSweepSummary) {

	rolloverRetryAfter := expireSweepRolloverRetryAfter(now)
	for _, productCode := range expireSweepProductCodes {
		index, ok := indexes[productCode]
		if !ok {
			continue
		}
		records, err := models.GetUnfilledOrdersWithoutExpireDate(table, productCode, rolloverRetryAfter, expireSweepMaxRecords)
		if err != nil {
			summary.errors++
			msg := fmt.Sprintf("🚨【expireSweep】expire_date未設定レコードの取得に失敗: table=%s product_code=%s err=%v",
				table, productCode, err)
			log.Println(msg)
			slackClient.PostMessage(msg, true)
			continue
		}
		log.Printf("【expireSweep】方式B table:%s product_code:%s 対象候補:%d件 rolloverRetryAfter(UTC):%s",
			table, productCode, len(records), rolloverRetryAfter.Format(time.RFC3339))

		for _, record := range records {
			if index.completedIDs[record.OrderID] {
				markSweptOrderFilled(record, summary)
				continue
			}
			if index.activeIDs[record.OrderID] {
				// 取引所側に生存している注文は失効ではない（手動発注の未約定注文はここで守られる）
				summary.skippedActive++
				log.Printf("【expireSweep】取引所側でACTIVEのため対象外: table=%s OrderID=%s %s",
					table, record.OrderID, record.ProductCode)
				continue
			}
			// COMPLETED一覧の遡り限界より古いレコードは、約定済みを失効と誤判定しうるため保留する
			if index.oldestCompleted.IsZero() || !record.Timestamp.After(index.oldestCompleted) {
				summary.pending++
				summary.pendingIDs = append(summary.pendingIDs,
					fmt.Sprintf("%s(%s/%s)", record.OrderID, record.ProductCode, table))
				log.Printf("【expireSweep】判定保留: table=%s OrderID=%s %s timestamp(UTC):%s oldestCompleted(UTC):%s",
					table, record.OrderID, record.ProductCode,
					formatSweepTime(record.Timestamp), formatSweepTime(index.oldestCompleted))
				continue
			}
			cancelSweptOrder(record, "B", now, summary)
		}
	}
}

// notifyExpireSweepResult は処理結果をSlackへ通知する。
func notifyExpireSweepResult(indexes map[string]*exchangeOrderIndex, summary *expireSweepSummary) {
	msg := fmt.Sprintf("【expireSweep】buy:%d件 sell:%d件 を CANCELLED / 約定判明:%d件 / 判定保留:%d件 / 取引所ACTIVEのため据置:%d件 / エラー:%d件",
		summary.cancelledBuy, summary.cancelledSell, summary.filled, summary.pending, summary.skippedActive, summary.errors)
	log.Println(msg)
	slackClient.PostMessage(msg, false)

	if summary.pending == 0 {
		return
	}
	limits := make([]string, 0, len(expireSweepProductCodes))
	for _, productCode := range expireSweepProductCodes {
		if index, ok := indexes[productCode]; ok {
			limits = append(limits, fmt.Sprintf("%s:%s", productCode, formatSweepTime(index.oldestCompleted)))
		}
	}
	ids := summary.pendingIDs
	if len(ids) > expireSweepPendingSampleSize {
		ids = ids[:expireSweepPendingSampleSize]
	}
	pendingMsg := fmt.Sprintf("🚨【expireSweep】判定保留 %d件: COMPLETED一覧の遡り限界(%s)より古いレコード。手動確認が必要 order_ids=[%s]",
		summary.pending, strings.Join(limits, " "), strings.Join(ids, ", "))
	log.Println(pendingMsg)
	slackClient.PostMessage(pendingMsg, true)
}

/*
expireSweepJob はスケジューラから呼ばれるエントリポイント。

スケジューラの時刻指定はシステムTZ（本番はJST）に依存するため、
TZの誤設定を運用で検知できるようジョブ冒頭でローカル時刻とUTCの双方をログ出力する。
失効判定の比較はすべてUTC同士で行う。
*/
func expireSweepJob(apiClient *bitflyer.APIClient) {
	now := time.Now().UTC()
	log.Printf("【expireSweep】start of job now(local):%s now(UTC):%s",
		time.Now().Format(time.RFC3339), now.Format(time.RFC3339))

	grace := expireSweepGrace()
	summary := &expireSweepSummary{}
	indexes := make(map[string]*exchangeOrderIndex, len(expireSweepProductCodes))

	for _, productCode := range expireSweepProductCodes {
		index, err := buildExchangeOrderIndex(apiClient, productCode)
		if err != nil {
			summary.errors++
			msg := fmt.Sprintf("🚨【expireSweep】注文一覧の取得に失敗したためsweepをスキップします: %v (product_code=%s)",
				err, productCode)
			log.Println(msg)
			slackClient.PostMessage(msg, true)
			continue
		}
		indexes[productCode] = index
	}

	if len(indexes) == 0 {
		log.Println("【expireSweep】有効な注文一覧が1件も取得できなかったため処理を中止します")
		log.Println("【expireSweep】end of job")
		return
	}

	// 方式A: expire_date の経過による失効検出（主）
	sweepExpiredOrders(models.TableBuyOrders, now, grace, indexes, summary)
	sweepExpiredOrders(models.TableSellOrders, now, grace, indexes, summary)

	// 方式B: expire_date 未設定の旧レコードを一覧からの消滅で補助判定
	sweepOrdersWithoutExpireDate(models.TableBuyOrders, now, indexes, summary)
	sweepOrdersWithoutExpireDate(models.TableSellOrders, now, indexes, summary)

	notifyExpireSweepResult(indexes, summary)
	log.Println("【expireSweep】end of job")
}
