package bitflyerApp

import (
	"fmt"
	"log"
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
    既定値7日は buy_minute_to_expire(10080分 = 7日)と整合させている。
  - ボット発注か手動発注かは区別しない。取引所に残っている未約定の買い注文は
    一律に同じ扱いとする（ユーザー確定仕様）。

以前は utils.BfCancelCriteria(-3日固定)で判定し、かつローカル時刻とDBのUTC時刻を
比較していたため9時間のずれが生じていた。本ジョブの比較はすべてUTCで行う。

キャンセルAPIが失敗した場合はDBを更新しない（キャンセルできていない注文を
CANCELLED として扱うと、実在する注文がスロット計算から消えて二重発注の温床になるため）。
*/

// cancelBuyOrderMaxRecords は1回の実行で判定する未約定買い注文の上限件数。
const cancelBuyOrderMaxRecords = 200

func cancelBuyOrderJob(apiClient *bitflyer.APIClient) {
	now := time.Now().UTC()
	log.Printf("【cancelBuyOrderJob】start of job now(local):%s now(UTC):%s",
		time.Now().Format(time.RFC3339), now.Format(time.RFC3339))

	cancelDays := config.Config.BFBuyOrderCancelDays
	if cancelDays <= 0 {
		cancelDays = config.DefaultBuyOrderCancelDays
	}
	threshold := now.AddDate(0, 0, -cancelDays)

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
	for _, order := range orders {
		// 既に失効している注文はキャンセルAPIを叩かない（expireSweepJobが処理する）
		if order.ExpireDate != nil && !order.ExpireDate.After(now) {
			log.Printf("【cancelBuyOrderJob】既に期限切れのためスキップ: OrderID=%s expire_date(UTC):%s",
				order.OrderID, formatSweepExpireDate(order.ExpireDate))
			continue
		}
		// 発注からの経過日数が閾値未満の注文は残す
		if order.Timestamp.After(threshold) {
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

	log.Printf("【cancelBuyOrderJob】end of job cancelled:%d failed:%d", cancelled, failed)
}
