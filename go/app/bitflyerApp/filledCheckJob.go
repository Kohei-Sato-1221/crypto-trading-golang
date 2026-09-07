package bitflyerApp

import (
	"fmt"
	"log"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/bitflyer"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

/*
filledCheckJob はDB上で未約定(UNFILLED)の注文が取引所側で約定していないかを確認し、
約定していれば FILLED に更新するジョブ。

取得件数は getchildorders の実効上限である bitflyer.MaxChildOrdersCount(500件)を明示指定する。
本ジョブは90秒周期で動くため、直近の約定は必ず1ページ目に含まれる。
*/
func filledCheckJob(productCode string, apiClient *bitflyer.APIClient) {
	log.Printf("【filledCheckJob】start of job %v", productCode)

	// DB上の未約定注文（buy_orders / sell_orders）を取得する
	ids, err := models.FilledCheck(productCode)
	if err != nil {
		msg := fmt.Sprintf("🚨【filledCheckJob】未約定注文の取得に失敗: product_code=%s err=%v", productCode, err)
		log.Println(msg)
		slackClient.PostMessage(msg, true)
		log.Printf("【filledCheckJob】end of job as error %v", productCode)
		return
	}
	if len(ids) == 0 {
		log.Printf("【filledCheckJob】end of job (no unfilled orders) %v", productCode)
		return
	}

	completedOrders, err := apiClient.GetChildOrders(bitflyer.GetChildOrdersParams{
		ProductCode:     productCode,
		ChildOrderState: "COMPLETED",
		Count:           bitflyer.MaxChildOrdersCount,
	})
	if err != nil {
		msg := fmt.Sprintf("🚨【filledCheckJob】約定済み注文一覧の取得に失敗: product_code=%s err=%v", productCode, err)
		log.Println(msg)
		slackClient.PostMessage(msg, true)
		log.Printf("【filledCheckJob】end of job as error %v", productCode)
		return
	}

	completedIDs := make(map[string]bool, len(completedOrders))
	for _, order := range completedOrders {
		completedIDs[order.ChildOrderAcceptanceID] = true
	}

	for i, orderID := range ids {
		log.Printf("No%d Id:%v", i, orderID)
		if !completedIDs[orderID] {
			continue
		}
		log.Printf("## filledCheckJob orderid:%v has been filled!", orderID)
		if err := models.UpdateFilledOrder(orderID); err != nil {
			// 1件の失敗で以降の注文が処理されなくならないよう continue する
			msg := fmt.Sprintf("🚨【filledCheckJob】約定ステータスの更新に失敗: OrderID=%s product_code=%s err=%v",
				orderID, productCode, err)
			log.Println(msg)
			slackClient.PostMessage(msg, true)
			continue
		}
		log.Printf("Order updated successfully!! orderId:%s", orderID)
	}
	log.Printf("【filledCheckJob】end of job %v", productCode)
}
