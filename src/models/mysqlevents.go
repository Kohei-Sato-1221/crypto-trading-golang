package models

import (
	"log"
	"strconv"
	"time"
)

type OkexOrderEvent struct {
	OrderID      string    `json:"order_id"`
	ClientOid    string    `json:"client_oid"`
	Type         string    `json:"type"`
	Side         string    `json:"side"`
	InstrumentID string    `json:"instrument_id"`
	OrderType    string    `json:"order_type"`
	Price        string    `json:"price"`
	Size         string    `json:"size"`
	State        string    `json:"state"`
	Timestamp    time.Time `json:"time"`
}

type OkexFilledBuyOrder struct {
	OrderID string  `json:"orderid"`
	Price   float64 `json:"price"`
	Size    float64 `json:"size"`
}

// OKEXからデータを取得して、DBと同期するメソッド
func SyncOkexBuyOrders(orders *[]OkexOrderEvent) {
	cmd1, _ := MysqlDbConn.Prepare("SELECT state FROM buy_orders WHERE orderid = ?")
	cmd2, _ := MysqlDbConn.Prepare("INSERT INTO buy_orders (orderid, pair, side, price, size, exchange, state) VALUES (?, ?, ?, ?, ?, ?, ?)")
	cmd3, _ := MysqlDbConn.Prepare("UPDATE buy_orders SET state = ? WHERE orderid = ?")
	defer cmd1.Close()
	defer cmd2.Close()
	defer cmd3.Close()
	for _, o := range *orders {
		log.Printf("orderid %v ", o.OrderID)
		rows, _ := cmd1.Query(o.OrderID)
		state := -99
		for rows.Next() {
			rows.Scan(&state)
		}
		if state == -99 {
			_, err := cmd2.Exec(o.OrderID, o.InstrumentID, o.Side, o.Price, o.Size, "okex", o.State)
			if err != nil {
				log.Println("Failure to do SyncBuyOrders.....")
			} else {
				log.Printf("orderid %v has been newly inserted!", o.OrderID)
			}
		} else if o.State != strconv.Itoa(state) {
			log.Printf("Update!!! orderid:%v", o.OrderID)
			_, err := cmd3.Exec(o.State, o.OrderID)
			if err != nil {
				log.Println("Failure to do SyncBuyOrders.....")
			} else {
				log.Printf("orderid %v has been updated!", o.OrderID)
			}
		}
	}
}

func UpdateOkexSellOrders(orderID string, sellPrice float64) {
	cmd1, _ := MysqlDbConn.Prepare("UPDATE buy_orders SET sellOrderState = 1, sellPrice = ? WHERE orderid = ?")
	defer cmd1.Close()
	_, err := cmd1.Exec(sellPrice, orderID)
	if err != nil {
		log.Println("Failure to do updateOkexSellOrders.....")
	} else {
		log.Printf("orderid %v : sell order updated!", orderID)
	}
}

func GetSoldBuyOrderList(pair string) []OkexFilledBuyOrder {
	log.Printf("GetSoldBuyOrderList: %v ", pair)
	cmd1, _ := MysqlDbConn.Prepare(`SELECT orderid, price, size FROM buy_orders WHERE state = 2 and sellOrderState = 0 and pair = ?`)
	log.Printf("cmd1 %v", cmd1)
	rows, err := cmd1.Query(pair)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var cnt int = 0
	var filledBuyOrders []OkexFilledBuyOrder
	for rows.Next() {
		var orderID string
		var price float64
		var size float64
		if err := rows.Scan(&orderID, &price, &size); err != nil {
			log.Println("Failure to get records.....")
			return nil
		}
		log.Printf("GetSold: %v %v %v", orderID, price, size)
		cnt++
		buyOrder := OkexFilledBuyOrder{OrderID: orderID, Price: price, Size: size}
		filledBuyOrders = append(filledBuyOrders, buyOrder)
	}
	return filledBuyOrders
}
