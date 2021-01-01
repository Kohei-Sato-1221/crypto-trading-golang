package okex

import (
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/models"
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
	OrderID string  `json:"order_id"`
	Price   float64 `json:"price"`
	Size    float64 `json:"size"`
}

//TODO structを整理すること
//TODO timestampはstring以外の型にすること
type BuyOrder struct {
	ID             uint    `gorm:"primary_key"`
	OrderID        string  `json:"order_id"`
	Pair           string  `json:"pair"`
	Price          float64 `json:"price"`
	Size           float64 `json:"size"`
	Exchange       string  `json:"exchange"`
	State          int     `json:"state"`
	SellOrderID    string  `json:"sell_order_id"`
	SellOrderState string  `json:"sell_order_state"`
	SellPrice      float64 `json:"sell_price"`
	Side           string  `json:"side"`
	Timestamp      string  `json:"timestamp"`
	Updatetime     string  `json:"updatetime"`
}

var TableName string

// OKEXからデータを取得して、DBと同期するメソッド
func SyncOkexBuyOrders(exchange string, orders *[]OkexOrderEvent) {
	cmd1, _ := models.AppDB.Prepare("SELECT state FROM " + TableName + " WHERE order_id = ?")
	cmd2, _ := models.AppDB.Prepare("INSERT INTO " + TableName + " (order_id, pair, side, price, size, exchange, state) VALUES (?, ?, ?, ?, ?, ?, ?)")
	cmd3, _ := models.AppDB.Prepare("UPDATE " + TableName + " SET state = ? WHERE order_id = ?")
	defer cmd1.Close()
	defer cmd2.Close()
	defer cmd3.Close()
	for _, o := range *orders {
		log.Printf("order_id %v ", o.OrderID)
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
				log.Printf("order_id %v has been newly inserted!", o.OrderID)
			}
		} else if o.State != strconv.Itoa(state) {
			log.Printf("Update!!! order_id:%v", o.OrderID)
			_, err := cmd3.Exec(o.State, o.OrderID)
			if err != nil {
				log.Println("Failure to do SyncBuyOrders.....")
			} else {
				log.Printf("order_id %v has been updated!", o.OrderID)
			}
		}
	}
}

func SyncOkexSellOrders(orders *[]OkexOrderEvent) {
	cmd1, _ := models.AppDB.Prepare("UPDATE " + TableName + " SET sell_order_state = 2 WHERE sell_order_id = ?")
	defer cmd1.Close()
	for _, o := range *orders {
		log.Printf("order_id %v ", o.OrderID)
		result, err := cmd1.Exec(o.OrderID)
		if err != nil {
			log.Println("Failure to do SyncOkexSellOrders.....")
		} else {
			lastInsertID, err2 := result.LastInsertId()
			if err2 != nil && lastInsertID != 0 {
				log.Printf("sell_order_id %v has been updated! result:%v", o.OrderID, result)
			}
		}
	}
}

//売り注文を発注した際にDBのレコードをアップデートする
func UpdateOkexSellOrders(order_id, sell_order_id string, sell_price float64) {
	cmd1, _ := models.AppDB.Prepare("UPDATE " + TableName + " SET sell_order_state = 1, sell_order_id = ?, sell_price = ? WHERE order_id = ?")
	defer cmd1.Close()
	_, err := cmd1.Exec(sell_order_id, sell_price, order_id)
	if err != nil {
		log.Println("Failure to do updateOkexSellOrders.....")
	} else {
		log.Printf("order_id %v : sell order updated!", order_id)
	}
}

func GetSoldBuyOrderList(pair string) []OkexFilledBuyOrder {
	log.Printf("GetSoldBuyOrderList: %v ", pair)
	cmd1, _ := models.AppDB.Prepare(`SELECT order_id, price, size FROM ` + TableName + ` WHERE state = 2 and sell_order_state = 0 and pair = ?`)
	log.Printf("cmd1 %v", cmd1)
	rows, err := cmd1.Query(pair)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var cnt int = 0
	var filledBuyOrders []OkexFilledBuyOrder
	for rows.Next() {
		var order_id string
		var price float64
		var size float64
		if err := rows.Scan(&order_id, &price, &size); err != nil {
			log.Println("Failure to get records.....")
			return nil
		}
		log.Printf("GetSold: %v %v %v", order_id, price, size)
		cnt++
		buyOrder := OkexFilledBuyOrder{OrderID: order_id, Price: price, Size: size}
		filledBuyOrders = append(filledBuyOrders, buyOrder)
	}
	return filledBuyOrders
}

//過去3日分の利益を取得する関数
func GetOKexResults() (string, error) {
	rows, err := models.AppDB.Query(`
		select
			'total' date,
			round(sum(average.profit) * 0.9988, 4) profit,
			count(average.profit) count,
			round(avg(average.profit) * 0.9988, 4)  ppt
		from
			(select
				DATE_FORMAT(updatetime, '%Y-%m-%d') date,
				sum((sell_price - price) * size * 106) profit
			from buy_orders
			where sell_order_state = 2
			group by DATE_FORMAT(updatetime, '%Y%m%d')
		) average
		union
		select
			DATE_FORMAT(updatetime, '%Y-%m-%d') date,
			round(sum((sell_price - price) * size * 106) * 0.9988, 4) profit,
			count(sell_price) count,
			round(sum((sell_price - price) * size * 106) / count(sell_price) * 0.9988, 4) ppt
		from buy_orders
		where sell_order_state = 2
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

func GetCancelledOrders() ([]BuyOrder, error) {
	buyOrders := []BuyOrder{}
	if err := models.GormDB.Limit(100).Where("state = ?", 0).Find(&buyOrders).Error; err != nil {
		return nil, errors.New("failed to do GetCancelledBuyOrders")
	}
	return buyOrders, nil
}

func UpdateCancelledOrder(order_id string) error {
	cmd, _ := models.AppDB.Prepare(`update buy_orders set state = -1, sell_order_state = -1 where order_id = ?`)
	_, err := cmd.Exec(order_id)
	if err != nil {
		return err
	}
	return nil
}
