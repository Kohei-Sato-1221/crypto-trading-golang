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
cancelBuyOrderJob は長期間約定しない買い注文を能動的にキャンセルしてスロットを解放するジョブ。

判定基準:
  - 有効期限(expire_date)を既に過ぎているレコードは対象外。
    取引所側では失効済みで注文が存在しないためキャンセルAPIは成功せず、
    DB上の後始末は expireSweepJob が行う。
  - 期限内でも、発注から buy_order_cancel_days 日を超えて約定しないものはキャンセルする。
  - timestamp を解釈できない（DB値のパース失敗）レコードは判定不能としてキャンセルしない。
    ゼロ値の Timestamp は After(threshold) が常に false になり「十分古い」と解釈されて
    キャンセルへ倒れてしまうため、明示的に除外して件数をSlack通知する。
  - ボット発注か手動発注かは区別しない。取引所に残っている未約定の買い注文は
    一律に同じ扱いとする（ユーザー確定仕様）。

# buy_order_cancel_days と buy_minute_to_expire の関係（F5・確定方針）

placeBuyOrder は expire_date = timestamp + buy_minute_to_expire を記録する。本ジョブは
上記のとおり「① expire_date <= now なら skip」→「② timestamp <= now - cancelDays なら cancel」
の順で判定するため、cancelDays * 1440 >= buy_minute_to_expire の設定では
②を満たすレコードが必ず①に吸収され、能動キャンセルは expire_date が NULL の
旧レコードにしか効かなくなる。

出荷設定(buy_minute_to_expire=10080分=7日 / buy_order_cancel_days=7日)は
まさにこの状態にあたる。これは意図した設定である:

  - 買い注文の期限を2.5日から7日へ延ばしたのは約定率を上げるためであり(S5)、
    7日より手前で能動キャンセルするとその狙いを削いでしまう。
  - 期限切れのスロット解放は expireSweepJob(06:05 JST)が翌朝に担保しており、
    能動キャンセルが無くてもスロットは滞留しない。

したがって本ジョブは「期限切れは sweep 任せ」という前提のもと、
  - expire_date が NULL の旧レコードの掃除
  - buy_order_cancel_days を buy_minute_to_expire/1440 より小さくチューニングしたときの
    早期スロット解放
の2つを担う。設定値の関係を取り違えるとジョブが黙って無効化されるため、
実行のたびに cancelBuyOrderConfigNote() で整合を点検し、どちらのモードで
動いているかをログに残す。

以前は固定日数(旧 utils.BfCancelCriteria = -3日。現在は撤去済み)で判定し、
かつローカル時刻とDBのUTC時刻を比較していたため9時間のずれが生じていた。
本ジョブの比較はすべてUTCで行う。

キャンセルAPIが失敗した場合はDBを更新しない（キャンセルできていない注文を
CANCELLED として扱うと、実在する注文がスロット計算から消えて二重発注の温床になるため）。
*/

const (
	// cancelBuyOrderMaxRecords は1回の実行で判定する未約定買い注文の上限件数。
	cancelBuyOrderMaxRecords = 200

	// cancelBuyOrderSampleSize は判定を見送ったレコードのSlack通知に載せる order_id の最大件数。
	cancelBuyOrderSampleSize = 10

	// minutesPerDay は buy_order_cancel_days(日) と buy_minute_to_expire(分) を突き合わせるための換算値。
	minutesPerDay = 1440
)

/*
cancelBuyOrderDecision は1レコードに対して本ジョブが取る処理。
*/
type cancelBuyOrderDecision int

const (
	// cancelBuyOrderKeep は判定日数に達していないため注文をそのまま残す。
	cancelBuyOrderKeep cancelBuyOrderDecision = iota
	// cancelBuyOrderCancel はキャンセルAPIを叩いてDBを CANCELLED にする。
	cancelBuyOrderCancel
	// cancelBuyOrderSkipExpired は既に失効しているため触らない（expireSweepJobの担当）。
	cancelBuyOrderSkipExpired
	// cancelBuyOrderSkipInvalidTimestamp は timestamp を解釈できず判定できない。
	cancelBuyOrderSkipInvalidTimestamp
)

/*
decideCancelBuyOrder は1件の未約定買い注文に対する処理を決める（API・DB・Slackに触れない純粋関数）。

判定順:
 1. expire_date を既に過ぎている → 取引所側では失効済みでキャンセルAPIが成功しないため触らない
 2. timestamp を解釈できない     → 判定不能。キャンセルしない側に倒す
    order.Timestamp.After(threshold) はゼロ値のとき常に false になり
    「十分古い」と解釈されてキャンセルへ進んでしまうため、明示的に除外する
    （方式B(sweep)が同じケースを判定保留にしているのと同じ扱いに揃える）
 3. 発注からの経過日数が閾値未満 → そのまま残す
 4. 上記以外                     → キャンセルする
*/
func decideCancelBuyOrder(order models.OrderRecord, now, threshold time.Time) cancelBuyOrderDecision {
	if order.ExpireDate != nil && !order.ExpireDate.After(now) {
		return cancelBuyOrderSkipExpired
	}
	if !order.TimestampValid || order.Timestamp.IsZero() {
		return cancelBuyOrderSkipInvalidTimestamp
	}
	if order.Timestamp.After(threshold) {
		return cancelBuyOrderKeep
	}
	return cancelBuyOrderCancel
}

/*
cancelBuyOrderConfigNote は buy_order_cancel_days と buy_minute_to_expire の整合を点検する
純粋関数（設定値だけを見る。API・DB・Slackに触れない）。

戻り値:
  - note      : どちらのモードで動いているかを説明するログ用の文言
  - effective : expire_date を持つレコードに対して能動キャンセルが発火しうるか

placeBuyOrder は expire_date = timestamp + buy_minute_to_expire を記録し、
decideCancelBuyOrder は「expire_date <= now なら skip」を「timestamp <= now - cancelDays なら cancel」
より先に評価する。したがって cancelDays * 1440 >= buy_minute_to_expire では
キャンセル条件を満たすレコードが必ず skip に吸収され、能動キャンセルは
expire_date が NULL の旧レコードにしか効かない（effective=false）。

出荷設定はこの effective=false 側であり、これは意図した状態である（F5・確定方針。
詳細はファイル冒頭のコメントを参照）。設定値を取り違えてジョブが黙って無効化される事故を
防ぐため、毎回ログに残す。
*/
func cancelBuyOrderConfigNote(cancelDays, buyMinuteToExpire int) (note string, effective bool) {
	if cancelDays <= 0 {
		cancelDays = config.DefaultBuyOrderCancelDays
	}
	if buyMinuteToExpire <= 0 {
		buyMinuteToExpire = config.DefaultBuyMinuteToExpire
	}

	cancelMinutes := cancelDays * minutesPerDay
	if cancelMinutes < buyMinuteToExpire {
		return fmt.Sprintf(
			"能動キャンセル有効: buy_order_cancel_days=%d日(%d分) < buy_minute_to_expire=%d分。"+
				"期限到来より前に未約定の買い注文をキャンセルしてスロットを早期解放する",
			cancelDays, cancelMinutes, buyMinuteToExpire), true
	}
	return fmt.Sprintf(
		"能動キャンセルは expire_date が NULL の旧レコードのみ対象: buy_order_cancel_days=%d日(%d分) >= buy_minute_to_expire=%d分。"+
			"キャンセル条件を満たすレコードは期限切れスキップに吸収されるため、期限切れの後始末は expireSweepJob が担当する"+
			"（F5・意図した設定。早期解放したい場合は buy_order_cancel_days を buy_minute_to_expire/1440 より小さくする）",
		cancelDays, cancelMinutes, buyMinuteToExpire), false
}

func cancelBuyOrderJob(apiClient *bitflyer.APIClient) {
	now := time.Now().UTC()
	log.Printf("【cancelBuyOrderJob】start of job now(local):%s now(UTC):%s",
		time.Now().Format(time.RFC3339), now.Format(time.RFC3339))

	cancelDays := config.Config.BFBuyOrderCancelDays
	if cancelDays <= 0 {
		cancelDays = config.DefaultBuyOrderCancelDays
	}
	threshold := now.AddDate(0, 0, -cancelDays)

	// 設定値の整合を毎回点検してログに残す。閾値を取り違えるとジョブが黙って無効化されるため（F5）
	configNote, cancelEffective := cancelBuyOrderConfigNote(cancelDays, config.Config.BFBuyMinuteToExpire)
	log.Printf("【cancelBuyOrderJob】設定点検: effective:%t %s", cancelEffective, configNote)

	orders, err := models.GetUnfilledBuyOrderRecords(cancelBuyOrderMaxRecords)
	if err != nil {
		msg := fmt.Sprintf("🚨【cancelBuyOrderJob】未約定買い注文の取得に失敗: err=%v", err)
		log.Println(msg)
		slackClient.PostMessage(msg, true)
		log.Println("【cancelBuyOrderJob】end of job as error")
		return
	}
	log.Printf("【cancelBuyOrderJob】対象候補:%d件 cancelDays:%d threshold(UTC):%s",
		len(orders), cancelDays, threshold.Format(time.RFC3339))

	cancelled := 0
	failed := 0
	var invalidTimestampIDs []string
	for _, order := range orders {
		switch decideCancelBuyOrder(order, now, threshold) {
		case cancelBuyOrderSkipExpired:
			// 既に失効している注文はキャンセルAPIを叩かない（expireSweepJobが処理する）
			log.Printf("【cancelBuyOrderJob】既に期限切れのためスキップ: OrderID=%s expire_date(UTC):%s",
				order.OrderID, formatSweepExpireDate(order.ExpireDate))
			continue
		case cancelBuyOrderSkipInvalidTimestamp:
			// timestamp を解釈できないレコードは経過日数を判定できない。
			// ゼロ値を「十分古い」と解釈してキャンセルしてしまわないよう明示的に除外する
			invalidTimestampIDs = append(invalidTimestampIDs, order.OrderID)
			log.Printf("【cancelBuyOrderJob】timestampを解釈できないため判定を見送り: OrderID=%s %s price=%.2f size=%v",
				order.OrderID, order.ProductCode, order.Price, order.Size)
			continue
		case cancelBuyOrderKeep:
			// 発注からの経過日数が閾値未満の注文は残す
			continue
		}

		cancelOrderParam := &bitflyer.Order{
			ProductCode:            order.ProductCode,
			ChildOrderAcceptanceID: order.OrderID,
		}
		if err := apiClient.CancelOrder(cancelOrderParam); err != nil {
			failed++
			msg := fmt.Sprintf("🚨【cancelBuyOrderJob】CancelOrder 失敗: OrderID=%s %s side=%s price=%.2f size=%v timestamp(UTC)=%s expire_date(UTC)=%s err=%v （DBは更新しません）",
				order.OrderID, order.ProductCode, order.Side, order.Price, order.Size,
				formatSweepTime(order.Timestamp), formatSweepExpireDate(order.ExpireDate), err)
			log.Println(msg)
			slackClient.PostMessage(msg, true)
			continue
		}

		if err := models.UpdateCancelledBuyOrder(order.OrderID); err != nil {
			failed++
			msg := fmt.Sprintf("🚨【cancelBuyOrderJob】キャンセルは成功しましたがDB更新に失敗: OrderID=%s %s price=%.2f size=%v err=%v",
				order.OrderID, order.ProductCode, order.Price, order.Size, err)
			log.Println(msg)
			slackClient.PostMessage(msg, true)
			continue
		}

		cancelled++
		msg := fmt.Sprintf("Cancelled BuyOrder: OrderId:%v %s price=%.2f size=%v timestamp(UTC)=%s",
			order.OrderID, order.ProductCode, order.Price, order.Size, formatSweepTime(order.Timestamp))
		log.Println(msg)
		slackClient.PostMessage(msg, true)
	}

	if len(invalidTimestampIDs) > 0 {
		ids := invalidTimestampIDs
		if len(ids) > cancelBuyOrderSampleSize {
			ids = ids[:cancelBuyOrderSampleSize]
		}
		msg := fmt.Sprintf("🚨【cancelBuyOrderJob】timestamp を解釈できず判定を見送った買い注文が %d件あります: order_ids=[%s]。"+
			"キャンセルしない側に倒しています。buy_orders.timestamp の値をご確認ください",
			len(invalidTimestampIDs), strings.Join(ids, ", "))
		log.Println(msg)
		slackClient.PostMessage(msg, true)
	}

	log.Printf("【cancelBuyOrderJob】end of job cancelled:%d failed:%d 判定見送り(timestamp不正):%d",
		cancelled, failed, len(invalidTimestampIDs))
}
