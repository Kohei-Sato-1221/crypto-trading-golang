package bitflyerApp

import (
	"fmt"
	"log"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/bitflyer"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

func placeSellOrder(apiClient *bitflyer.APIClient) {
	log.Println("【sellOrderjob】start of job")
	buyOrderInfos := models.CheckFilledBuyOrders()
	if buyOrderInfos == nil {
		log.Println("【sellOrderjob】 : No order ids ")
		return
	}

	for i, buyOrderInfo := range buyOrderInfos {
		orderID := buyOrderInfo.OrderID
		productCode := buyOrderInfo.ProductCode
		size := buyOrderInfo.Size
		sellPrice := buyOrderInfo.CalculateSellOrderPrice()
		log.Printf("No%d Id:%v sellPrice:%10.2f strategy:%v", i, orderID, sellPrice, buyOrderInfo.Strategy)

		sellOrder := &bitflyer.Order{
			ProductCode:     productCode,
			ChildOrderType:  "LIMIT",
			Side:            "SELL",
			Price:           sellPrice,
			Size:            size,
			MinuteToExpires: 43200, // 30days
			TimeInForce:     "GTC",
		}

		log.Printf("sell order:%v\n", sellOrder)
		res, err := apiClient.PlaceOrder(sellOrder)
		log.Printf("sell res:%v\n", res)
		if err != nil {
			errMsg := fmt.Sprintf("SellOrder failed.... Failure in [apiClient.PlaceOrder()] err:%v (BuyOrderID: %s, BuyPrice: %.2f, Strategy: %v)", err, orderID, buyOrderInfo.Price, buyOrderInfo.Strategy)
			log.Println(errMsg)
			slackClient.PostMessage(errMsg, true)
			break
		}
		if res == nil {
			errMsg := fmt.Sprintf("SellOrder failed.... no response (BuyOrderID: %s, BuyPrice: %.2f, Strategy: %v)", orderID, buyOrderInfo.Price, buyOrderInfo.Strategy)
			log.Println(errMsg)
			slackClient.PostMessage(errMsg, true)
			break
		}
		// Check for API error response (e.g., Insufficient funds)
		if res.Status != 0 || res.OrderId == "" {
			var errMsg string
			if res.ErrorMessage != "" {
				errMsg = fmt.Sprintf("SellOrder failed: %s (Status: %d, BuyOrderID: %s, BuyPrice: %.2f, Strategy: %v, SellProductCode: %s, SellPrice: %.2f, SellSize: %v)", res.ErrorMessage, res.Status, orderID, buyOrderInfo.Price, buyOrderInfo.Strategy, productCode, sellPrice, size)
			} else {
				errMsg = fmt.Sprintf("SellOrder failed: No order ID returned (Status: %d, BuyOrderID: %s, BuyPrice: %.2f, Strategy: %v, SellProductCode: %s, SellPrice: %.2f, SellSize: %v)", res.Status, orderID, buyOrderInfo.Price, buyOrderInfo.Strategy, productCode, sellPrice, size)
			}
			log.Println(errMsg)
			slackClient.PostMessage(errMsg, true)
			break
		}

		err = models.UpdateFilledOrderWithBuyOrder(orderID)
		if err != nil {
			log.Println("Failure to update records..... / #UpdateFilledOrderWithBuyOrder")
			break
		}
		log.Printf("Buy Order updated successfully!! #UpdateFilledOrderWithBuyOrder  orderId:%s", orderID)

		utc, _ := time.LoadLocation("UTC")
		utcCurrentDate := time.Now().In(utc)
		event := models.OrderEvent{
			OrderID:     res.OrderId,
			Time:        utcCurrentDate,
			ProductCode: productCode,
			Side:        "Sell",
			Price:       sellPrice,
			Size:        size,
			Exchange:    "bitflyer",
		}
		err = event.SellOrder(orderID)
		if err != nil {
			log.Println("BuyOrder failed.... Failure in [event.BuyOrder()]")
		} else {
			log.Printf("BuyOrder Succeeded! OrderId:%v", res.OrderId)
		}
	}
	log.Println("【sellOrderjob】end of job")
}
