package controller

import (
//	"strconv"
	"bitflyer"
	"config"
//	"fmt"
	"models"
	"github.com/carlescere/scheduler"
	"log"
	"time"
)

func StreamIngestionData() {
	var tickerChannl = make(chan bitflyer.Ticker)
	apiClient := bitflyer.New(config.Config.ApiKey, config.Config.ApiSecret)
	go apiClient.GetRealTimeTicker(config.Config.ProductCode, tickerChannl)
	
	buyingJob := func(){
//		ticker, _ := apiClient.GetTicker("BTC_JPY")
		e := models.OrderEvent{
			OrderId:     "or12234",
			Time:        time.Now(),
			ProductCode: "BTC_JPY",
			Side:        "BUY",
			Price:       100000.0,
			Size:        0.001,
			Exchange:    "bitflyer",
		}
		isSuccessfull := e.BuyOrder()
//		log.Printf("BTC price :%s", strconv.FormatFloat(ticker.GetMiddlePrice(), 'f', 4, 64))
		log.Printf("BuyOrder :%s", isSuccessfull)
	}
	
	sellingJob := func(){
//		fmt.Println("sell")
	}
	
	scheduler.Every(10).Seconds().Run(buyingJob)
	scheduler.Every(5).Seconds().Run(sellingJob)
}

/*
【jobの種類】

1.buyOrderJob:
・指定の周期で買い注文を発注するジョブ
・買い注文が発注したら以下のデータをinsertする
　[Table:buyorder]orderid, pair, volume, price, orderdate, exchange, filled 

2.filledCheckJob:
・指定の周期で買い注文の約定具合をチェックするジョブ
・買い注文が約定していた場合、buyorderテーブルのfilledをtrueにする

3.sellOrderJob:
・指定の周期で売り注文を発注するジョブ
・buyoerderのレコードでfilledがtrueかつ、sellorderに該当のorderidがない場合売り注文を出す。
・売り注文が発注できたら以下のデータをinsertする。
　[Table:sellorder]buyorderid, orderid, pair, volume, price, orderdate, exchange, filled 


*/




