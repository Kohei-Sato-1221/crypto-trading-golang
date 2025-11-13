package bitflyerApp

import (
	"log"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/bitflyer"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

func filledCheckJob(productCode string, apiClient *bitflyer.APIClient) {
	log.Println("【filledCheckJob】start of job %v", productCode)
	// Get list of unfilled buy orders in local Database(buy_orders & sell_orders)
	ids, err1 := models.FilledCheck(productCode)
	completed_orders, err2 := apiClient.GetActiveBuyOrders(productCode, "COMPLETED")
	if err1 != nil || err2 != nil {
		log.Println("error in filledCheckJob..... e1:%v  e2:%v", err1, err2)
		goto ENDOFFILLEDCHECK
	}

	if ids == nil {
		goto ENDOFFILLEDCHECK
	}

	// check if an order is filled for each orders calling API
	for i, orderId := range ids {
		log.Printf("No%d Id:%v", i, orderId)
		// order, err := apiClient.GetOrderByOrderId(orderId, productCode)
		orderIdExist := false
		for _, order := range *completed_orders {
			if orderId == order.ChildOrderAcceptanceID {
				orderIdExist = true
				log.Printf("## filledCheckJob  orderid:%v has been filled!")
				break
			}
		}
		if orderIdExist {
			err := models.UpdateFilledOrder(orderId)
			if err != nil {
				log.Println("Failure to update records.....")
				break
			}
			log.Printf("Order updated successfully!! orderId:%s", orderId)
		}
	}
ENDOFFILLEDCHECK:
	log.Println("【filledCheckJob】end of job %v", productCode)
}
