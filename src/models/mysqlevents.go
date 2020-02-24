package models

import (
	"fmt"
	"log"
	"time"
)

type OkexOrderEvent struct {
	OrderId      string    `json:"order_id"`
	ClientOid    string    `json:"client_oid"`
	Type         string    `json:"type"`
	Side         string    `json:"side"`
	InstrumentId string    `json:"instrument_id"`
	OrderType    string    `json:"order_type"`
	Price        string    `json:"price"`
	Size         string    `json:"size"`
	State        string    `json:"state"`
	Timestamp    time.Time `json:"time"`
}

func SyncOkexBuyOrders(orders *[]OkexOrderEvent) {
	cmd1, _ := MysqlDbConn.Prepare(fmt.Sprintf("SELECT COUNT(*) FROM buy_orders WHERE orderid = ?"))
	cmd2, _ := MysqlDbConn.Prepare(fmt.Sprintf("INSERT INTO buy_orders (orderid, pair, side, price, size, exchange, state) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"))
	for _, o := range *orders {
		log.Printf("orderid %v ", o.OrderId)
		rows := cmd1.QueryRow(o.OrderId)
		cnt := 0
		if rows != nil {
			rows.Scan(&cnt)
		}
		if cnt == 0 {
			log.Printf("xxxxxxxx3")
			if cmd2 == nil {
				log.Printf("cmd2 is nil")
				// TODO：ここでなぜかぬるぽが発生！
			}
			_, err := cmd2.Exec(o.OrderId, o.InstrumentId, o.Side, o.Price, o.Size, "okex", o.State)
			log.Printf("xxxxxxxx4")
			if err != nil {
				log.Println("Failure to do SyncBuyOrders.....")
			} else {
				log.Printf("orderid %v has been newly inserted!", o.OrderId)
			}
		}
	}
}
