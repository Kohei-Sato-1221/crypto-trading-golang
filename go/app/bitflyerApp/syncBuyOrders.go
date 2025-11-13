package bitflyerApp

import (
	"log"
	"strings"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/bitflyer"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

func syncBuyOrders(product_code string, apiClient *bitflyer.APIClient) {
	active_orders, err := apiClient.GetActiveBuyOrders(product_code, "ACTIVE")
	completed_orders, err := apiClient.GetActiveBuyOrders(product_code, "COMPLETED")
	if err != nil {
		log.Println("GetActiveOrders failed....")
	}
	var orderEvents []models.OrderEvent
	utc, _ := time.LoadLocation("UTC")
	utc_current_date := time.Now().In(utc)
	for _, order := range *active_orders {
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
			}
			orderEvents = append(orderEvents, event)
			log.Printf("【order】%v", event)
		}
	}
	// Completedされた注文に関しては2日以内に約定した注文のみ同期
	for _, order := range *completed_orders {
		utc, _ := time.LoadLocation("UTC")
		utc_current_date := time.Now().In(utc)
		compareOrderDate, _ := time.ParseInLocation("2006-01-02 15:04:05", strings.Replace(order.ChildOrderDate, "T", " ", 1), time.UTC)
		compareOrderDate = compareOrderDate.Add(60 * time.Minute)
		// compareOrderDate = compareOrderDate.Add(2880 * time.Minute)
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
			}
			orderEvents = append(orderEvents, event)
			log.Printf("【order】%v", event)
		}
	}
	models.SyncBuyOrders(&orderEvents)
}
