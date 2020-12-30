package models

import (
	"log"
	"strconv"
	"strings"
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

var TableName string

// OKEXからデータを取得して、DBと同期するメソッド
func SyncOkexBuyOrders(exchange string, orders *[]OkexOrderEvent) {
	cmd1, _ := MysqlDbConn.Prepare("SELECT state FROM " + TableName + " WHERE orderid = ?")
	cmd2, _ := MysqlDbConn.Prepare("INSERT INTO " + TableName + " (orderid, pair, side, price, size, exchange, state) VALUES (?, ?, ?, ?, ?, ?, ?)")
	cmd3, _ := MysqlDbConn.Prepare("UPDATE " + TableName + " SET state = ? WHERE orderid = ?")
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
			_, err := cmd2.Exec(o.OrderID, o.InstrumentID, o.Side, o.Price, o.Size, exchange, o.State)
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

func SyncOkexSellOrders(orders *[]OkexOrderEvent) {
	cmd1, _ := MysqlDbConn.Prepare("UPDATE " + TableName + " SET sellOrderState = 2 WHERE sellOrderId = ?")
	defer cmd1.Close()
	for _, o := range *orders {
		log.Printf("orderid %v ", o.OrderID)
		result, err := cmd1.Exec(o.OrderID)
		if err != nil {
			log.Println("Failure to do SyncOkexSellOrders.....")
		} else {
			lastInsertID, err2 := result.LastInsertId()
			if err2 != nil && lastInsertID != 0 {
				log.Printf("sellorderid %v has been updated! result:%v", o.OrderID, result)
			}
		}
	}
}

//売り注文を発注した際にDBのレコードをアップデートする
func UpdateOkexSellOrders(orderID, sellOrderId string, sellPrice float64) {
	cmd1, _ := MysqlDbConn.Prepare("UPDATE " + TableName + " SET sellOrderState = 1, sellOrderId = ?, sellPrice = ? WHERE orderid = ?")
	defer cmd1.Close()
	_, err := cmd1.Exec(sellOrderId, sellPrice, orderID)
	if err != nil {
		log.Println("Failure to do updateOkexSellOrders.....")
	} else {
		log.Printf("orderid %v : sell order updated!", orderID)
	}
}

func GetSoldBuyOrderList(pair string) []OkexFilledBuyOrder {
	log.Printf("GetSoldBuyOrderList: %v ", pair)
	cmd1, _ := MysqlDbConn.Prepare(`SELECT orderid, price, size FROM ` + TableName + ` WHERE state = 2 and sellOrderState = 0 and pair = ?`)
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

//過去3日分の利益を取得する関数
func GetOKexResults() (string, error) {
	rows, err := MysqlDbConn.Query(`
		select
			'total' date,
			round(sum(average.profit) * 0.9988, 4) profit,
			count(average.profit) count,
			round(avg(average.profit) * 0.9988, 4)  ppt
		from
			(select
				DATE_FORMAT(updatetime, '%Y-%m-%d') date,
				sum((sellPrice - price) * size * 106) profit
			from buy_orders
			where sellOrderState = 2
			group by DATE_FORMAT(updatetime, '%Y%m%d')
		) average
		union
		select
			DATE_FORMAT(updatetime, '%Y-%m-%d') date,
			round(sum((sellPrice - price) * size * 106) * 0.9988, 4) profit,
			count(sellPrice) count,
			round(sum((sellPrice - price) * size * 106) / count(sellPrice) * 0.9988, 4) ppt
		from buy_orders
		where sellOrderState = 2
		group by DATE_FORMAT(updatetime, '%Y%m%d')
		order by date desc
		limit 4
		`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var sb strings.Builder
	sb.WriteString("【okex 自動売買 収益】\n")
	sb.WriteString("date / profit / count / ppt\n")
	for rows.Next() {
		var date string
		var profit string
		var count string
		var ppt string

		if err := rows.Scan(&date, &profit, &count, &ppt); err != nil {
			log.Println("Failure to get records.....")
			return "", err
		}
		sb.WriteString(date + " / " + profit + " / " + count + " / " + ppt + "\n")
	}
	return sb.String(), nil
}
