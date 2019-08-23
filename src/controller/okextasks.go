package controller

import (
	"config"
	"log"
	"okex"
	//"runtime"
)

func StartOKEXService() {
	log.Println("【StartOKEXService】")
	apiClient := okex.New(config.Config.OKApiKey, config.Config.OKApiSecret, config.Config.OKPassPhrase)	
	
	buyOrder := &okex.Order{
		ClientOid:      "SugarOrder1221",
		Type:           "limit",
		Side:           "buy",
		InstrumentId:   "ETH-USDT",
		OrderType:      "1",
		Price:          "182.10",
		Size:           "0.01",
	}
	
	log.Printf("buyorder:%v\n", buyOrder)
	res, err := apiClient.PlaceOrder(buyOrder)
	if err != nil{
		log.Println("Buyorder failed.... Failure in [apiClient.PlaceOrder()]")
		return
	}
	if res == nil{
		log.Println("Buyorder failed.... no response")
		return
	}else{
		log.Println("Buyorder response %v %s", res)
	}
//	apiClient.ShowParams()
	//runtime.Goexit()
}

//type Order struct {
//	ClientOid      string  `json:"client_oid"`
//	Type           string  `json:"type"`
//	Side           string  `json:"side"`
//	InstrumentId   string  `json:"instrument_id"`
//	OrderType      string  `json:"order_type"`
//	Price          string  `json:"price"`
//	Size           string  `json:"size"`
//} 


