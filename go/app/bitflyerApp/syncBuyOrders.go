package bitflyerApp

import (
	"log"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/bitflyer"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/utils"
)

/*
parseOrderExpireDate は取引所APIが返す expire_date をUTCのtime.Timeにパースして返す。

Bitflyerの expire_date はタイムゾーンサフィックスの無いUTC・秒精度の文字列
（実APIレスポンスで確認済み: "2026-10-05T22:41:05"）。
パースできない場合は nil を返し、DB側の expire_date は更新しない（誤った期限で
失効判定されるより、期限不明のまま補助判定に委ねる方が安全なため）。
*/
func parseOrderExpireDate(order bitflyer.Order) *time.Time {
	if order.ExpireDate == "" {
		return nil
	}
	expireDate, err := utils.ParseBitflyerTime(order.ExpireDate)
	if err != nil {
		log.Printf("[ERROR] failed to parse expire_date. order_id:%s expire_date:%s err:%v",
			order.ChildOrderAcceptanceID, order.ExpireDate, err)
		return nil
	}
	return &expireDate
}

// logSyncedOrder は同期対象の注文をログ出力する。
// expire_date はポインタのためアドレスが出ないよう明示的に文字列化する。
func logSyncedOrder(event models.OrderEvent) {
	expire := "nil"
	if event.ExpireDate != nil {
		expire = event.ExpireDate.Format(time.RFC3339)
	}
	log.Printf("【order】order_id:%s product_code:%s side:%s price:%10.2f size:%v status:%s expire_date(UTC):%s",
		event.OrderID, event.ProductCode, event.Side, event.Price, event.Size, event.Status, expire)
}

func syncBuyOrders(product_code string, apiClient *bitflyer.APIClient) {
	active_orders, err := apiClient.GetChildOrders(bitflyer.GetChildOrdersParams{
		ProductCode:     product_code,
		ChildOrderState: "ACTIVE",
	})
	if err != nil {
		log.Printf("GetChildOrders failed (ACTIVE): %v", err)
		return
	}
	var completed_orders []bitflyer.Order
	completed_orders, err = apiClient.GetChildOrders(bitflyer.GetChildOrdersParams{
		ProductCode:     product_code,
		ChildOrderState: "COMPLETED",
	})
	if err != nil {
		log.Printf("GetChildOrders failed (COMPLETED): %v", err)
		return
	}
	var orderEvents []models.OrderEvent
	utc, _ := time.LoadLocation("UTC")
	utc_current_date := time.Now().In(utc)
	for _, order := range active_orders {
		if order.Side == "BUY" {
			event := models.OrderEvent{
				OrderID:     order.ChildOrderAcceptanceID,
				Time:        utc_current_date,
				ProductCode: order.ProductCode,
				Side:        order.Side,
				Price:       order.Price,
				Size:        order.Size,
				Exchange:    "bitflyer",
				Status:      models.OrderStatusUnfilled,
				ExpireDate:  parseOrderExpireDate(order),
			}
			orderEvents = append(orderEvents, event)
			logSyncedOrder(event)
		}
	}
	// Completedされた注文に関しては2日以内に約定した注文のみ同期
	for _, order := range completed_orders {
		utc, _ := time.LoadLocation("UTC")
		utc_current_date := time.Now().In(utc)
		childOrderDate, err := utils.ParseBitflyerTime(order.ChildOrderDate)
		if err != nil {
			log.Printf("[ERROR] failed to parse child_order_date. order_id:%s child_order_date:%s err:%v",
				order.ChildOrderAcceptanceID, order.ChildOrderDate, err)
			continue
		}
		compareOrderDate := childOrderDate.Add(60 * time.Minute)
		if order.Side == "BUY" && compareOrderDate.After(utc_current_date) {
			event := models.OrderEvent{
				OrderID:     order.ChildOrderAcceptanceID,
				Time:        utc_current_date,
				ProductCode: order.ProductCode,
				Side:        order.Side,
				Price:       order.Price,
				Size:        order.Size,
				Exchange:    "bitflyer",
				Status:      models.OrderStatusFilled,
				ExpireDate:  parseOrderExpireDate(order),
			}
			orderEvents = append(orderEvents, event)
			logSyncedOrder(event)
		}
	}
	models.SyncBuyOrders(&orderEvents)
}
