package bitflyerApp

import (
	"fmt"
	"log"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/bitflyer"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/config"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

func placeSellOrder(apiClient *bitflyer.APIClient) {
	log.Println("【sellOrderjob】start of job")
	buyOrderInfos := models.CheckFilledBuyOrders()
	if buyOrderInfos == nil {
		log.Println("【sellOrderjob】 : No order ids ")
		return
	}

	// 売り注文の有効期限(分)。Bitflyerの上限は43200分(30日)
	sellMinuteToExpire := config.Config.BFSellMinuteToExpire
	if sellMinuteToExpire <= 0 {
		sellMinuteToExpire = config.DefaultSellMinuteToExpire
	}

	for i, buyOrderInfo := range buyOrderInfos {
		orderID := buyOrderInfo.OrderID
		productCode := buyOrderInfo.ProductCode
		size := buyOrderInfo.Size
		sellPrice := buyOrderInfo.CalculateSellOrderPrice()
		// 利確率は買い戦略ごとに変わる。マッピングに無い戦略値は既定値にフォールバックする
		profitRate, knownStrategy := buyOrderInfo.SellProfitRate()
		profitRateLog := fmt.Sprintf("%.3f", profitRate)
		if !knownStrategy {
			profitRateLog += "(fallback)"
		}
		log.Printf("No%d Id:%v buyPrice:%10.2f sellPrice:%10.2f strategy:%v profitRate:%s",
			i, orderID, buyOrderInfo.Price, sellPrice, buyOrderInfo.Strategy, profitRateLog)

		sellOrder := &bitflyer.Order{
			ProductCode:     productCode,
			ChildOrderType:  "LIMIT",
			Side:            "SELL",
			Price:           sellPrice,
			Size:            size,
			MinuteToExpires: sellMinuteToExpire,
			TimeInForce:     "GTC",
		}

		log.Printf("sell order:%v\n", sellOrder)
		res, err := apiClient.PlaceOrder(sellOrder)
		log.Printf("sell res:%v\n", res)
		if err != nil {
			errMsg := fmt.Sprintf("SellOrder failed.... Failure in [apiClient.PlaceOrder()] err:%v (BuyOrderID: %s, BuyPrice: %.2f, Strategy: %v, ProfitRate: %s, SellProductCode: %s, SellPrice: %.2f, SellSize: %v)",
				err, orderID, buyOrderInfo.Price, buyOrderInfo.Strategy, profitRateLog, productCode, sellPrice, size)
			log.Println(errMsg)
			slackClient.PostMessage(errMsg, true)
			continue
		}
		if res == nil {
			errMsg := fmt.Sprintf("SellOrder failed.... no response (BuyOrderID: %s, BuyPrice: %.2f, Strategy: %v, ProfitRate: %s, SellProductCode: %s, SellPrice: %.2f, SellSize: %v)",
				orderID, buyOrderInfo.Price, buyOrderInfo.Strategy, profitRateLog, productCode, sellPrice, size)
			log.Println(errMsg)
			slackClient.PostMessage(errMsg, true)
			continue
		}
		// Check for API error response (e.g., Insufficient funds)
		if res.Status != 0 || res.OrderId == "" {
			var errMsg string
			if res.ErrorMessage != "" {
				errMsg = fmt.Sprintf("SellOrder failed: %s (Status: %d, BuyOrderID: %s, BuyPrice: %.2f, Strategy: %v, ProfitRate: %s, SellProductCode: %s, SellPrice: %.2f, SellSize: %v)", res.ErrorMessage, res.Status, orderID, buyOrderInfo.Price, buyOrderInfo.Strategy, profitRateLog, productCode, sellPrice, size)
			} else {
				errMsg = fmt.Sprintf("SellOrder failed: No order ID returned (Status: %d, BuyOrderID: %s, BuyPrice: %.2f, Strategy: %v, ProfitRate: %s, SellProductCode: %s, SellPrice: %.2f, SellSize: %v)", res.Status, orderID, buyOrderInfo.Price, buyOrderInfo.Strategy, profitRateLog, productCode, sellPrice, size)
			}
			log.Println(errMsg)
			slackClient.PostMessage(errMsg, true)
			continue
		}

		err = models.UpdateFilledOrderWithBuyOrder(orderID)
		if err != nil {
			// 取引所には売り注文が出ている状態でDB更新に失敗している。
			// 次回以降の sellOrderjob で同じ買い注文が再度対象になり得るため、手動確認を促す
			errMsg := fmt.Sprintf("SellOrder failed.... Failure in [models.UpdateFilledOrderWithBuyOrder()] err:%v ※売り注文は発注済みのため要手動確認 (SellOrderID: %s, BuyOrderID: %s, ProductCode: %s, SellPrice: %.2f, SellSize: %v, Strategy: %v, ProfitRate: %s)",
				err, res.OrderId, orderID, productCode, sellPrice, size, buyOrderInfo.Strategy, profitRateLog)
			log.Println(errMsg)
			slackClient.PostMessage(errMsg, true)
			continue
		}
		log.Printf("Buy Order updated successfully!! #UpdateFilledOrderWithBuyOrder  orderId:%s", orderID)

		utc, _ := time.LoadLocation("UTC")
		utcCurrentDate := time.Now().In(utc)
		// 注文の有効期限(UTC)を記録する。ローリング(期限3日前の再発注)の判定に使う
		expireDate := utcCurrentDate.Add(time.Duration(sellMinuteToExpire) * time.Minute)
		event := models.OrderEvent{
			OrderID:     res.OrderId,
			Time:        utcCurrentDate,
			ProductCode: productCode,
			Side:        "Sell",
			Price:       sellPrice,
			Size:        size,
			Exchange:    "bitflyer",
		}
		err = event.SellOrderWithMeta(orderID, "", &expireDate)
		if err != nil {
			errMsg := fmt.Sprintf("SellOrder failed.... Failure in [event.SellOrderWithMeta()] err:%v (SellOrderID: %s, BuyOrderID: %s, ProductCode: %s, SellPrice: %.2f, SellSize: %v, Strategy: %v)",
				err, res.OrderId, orderID, productCode, sellPrice, size, buyOrderInfo.Strategy)
			log.Println(errMsg)
			slackClient.PostMessage(errMsg, true)
		} else {
			log.Printf("SellOrder Succeeded! OrderId:%v expire(UTC):%s", res.OrderId, expireDate.Format(time.RFC3339))
		}
	}
	log.Println("【sellOrderjob】end of job")
}
