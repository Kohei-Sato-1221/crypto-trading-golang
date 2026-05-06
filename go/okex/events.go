package okex

import (
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/database"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
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

// TODO structを整理すること
// TODO timestampはstring以外の型にすること
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

func (o *OkjBuyOrder) ConverToBuyOrder() *BuyOrder {
	return &BuyOrder{
		o.ID,
		o.OrderID,
		o.Pair,
		o.Price,
		o.Size,
		o.Exchange,
		o.State,
		o.SellOrderID,
		o.SellOrderState,
		o.SellPrice,
		o.Side,
		o.Timestamp,
		o.Updatetime,
	}
}

type OkjBuyOrder struct {
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
	var selectQuery, insertQuery, updateQuery string
	if database.CurrentDriver() == "postgres" {
		selectQuery = "SELECT state FROM " + TableName + " WHERE order_id = $1"
		insertQuery = "INSERT INTO " + TableName + " (order_id, pair, side, price, size, exchange, state) VALUES ($1, $2, $3, $4, $5, $6, $7)"
		updateQuery = "UPDATE " + TableName + " SET state = $1 WHERE order_id = $2"
	} else {
		selectQuery = "SELECT state FROM " + TableName + " WHERE order_id = ?"
		insertQuery = "INSERT INTO " + TableName + " (order_id, pair, side, price, size, exchange, state) VALUES (?, ?, ?, ?, ?, ?, ?)"
		updateQuery = "UPDATE " + TableName + " SET state = ? WHERE order_id = ?"
	}

	for _, o := range *orders {
		log.Printf("order_id %v ", o.OrderID)
		var state int
		err := models.AppDB.QueryRow(selectQuery, o.OrderID).Scan(&state)
		if err != nil {
			// レコードが見つからない場合はINSERT
			_, err := models.AppDB.Exec(insertQuery, o.OrderID, o.InstrumentID, o.Side, o.Price, o.Size, exchange, o.State)
			if err != nil {
				log.Println("Failure to do SyncBuyOrders.....")
			} else {
				log.Printf("order_id %v has been newly inserted!", o.OrderID)
			}
		} else if o.State != strconv.Itoa(state) {
			log.Printf("Update!!! order_id:%v", o.OrderID)
			_, err := models.AppDB.Exec(updateQuery, o.State, o.OrderID)
			if err != nil {
				log.Println("Failure to do SyncBuyOrders.....")
			} else {
				log.Printf("order_id %v has been updated!", o.OrderID)
			}
		}
	}
}

func SyncOkexSellOrders(orders *[]OkexOrderEvent) {
	var query string
	if database.CurrentDriver() == "postgres" {
		query = "UPDATE " + TableName + " SET sell_order_state = 2 WHERE sell_order_id = $1"
	} else {
		query = "UPDATE " + TableName + " SET sell_order_state = 2 WHERE sell_order_id = ?"
	}
	for _, o := range *orders {
		log.Printf("order_id %v ", o.OrderID)
		result, err := models.AppDB.Exec(query, o.OrderID)
		if err != nil {
			log.Println("Failure to do SyncOkexSellOrders.....")
		} else {
			rowsAffected, _ := result.RowsAffected()
			if rowsAffected > 0 {
				log.Printf("sell_order_id %v has been updated! rows_affected:%v", o.OrderID, rowsAffected)
			}
		}
	}
}

// 売り注文を発注した際にDBのレコードをアップデートする
func UpdateOkexSellOrders(order_id, sell_order_id string, sell_price, sell_size float64) {
	var query string
	if database.CurrentDriver() == "postgres" {
		if len(sell_order_id) == 0 {
			query = "UPDATE " + TableName + " SET sell_order_state = -1, sell_order_id = $1, sell_price = $2, sell_size = $3 WHERE order_id = $4"
		} else {
			query = "UPDATE " + TableName + " SET sell_order_state = 1, sell_order_id = $1, sell_price = $2, sell_size = $3 WHERE order_id = $4"
		}
	} else {
		if len(sell_order_id) == 0 {
			query = "UPDATE " + TableName + " SET sell_order_state = -1, sell_order_id = ?, sell_price = ?, sell_size = ? WHERE order_id = ?"
		} else {
			query = "UPDATE " + TableName + " SET sell_order_state = 1, sell_order_id = ?, sell_price = ?, sell_size = ? WHERE order_id = ?"
		}
	}

	_, err := models.AppDB.Exec(query, sell_order_id, sell_price, sell_size, order_id)
	if err != nil {
		log.Println("Failure to do updateOkexSellOrders.....")
	} else {
		log.Printf("order_id %v : sell order updated!", order_id)
	}
}

func GetSoldBuyOrderList(pair string) []OkexFilledBuyOrder {
	log.Printf("GetSoldBuyOrderList: %v ", pair)
	var query string
	if database.CurrentDriver() == "postgres" {
		query = `SELECT order_id, price, size FROM ` + TableName + ` WHERE state = 2 and sell_order_state = 0 and pair = $1`
	} else {
		query = `SELECT order_id, price, size FROM ` + TableName + ` WHERE state = 2 and sell_order_state = 0 and pair = ?`
	}
	rows, err := models.AppDB.Query(query, pair)
	if err != nil {
		return nil
	}
	defer rows.Close()

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
		buyOrder := OkexFilledBuyOrder{OrderID: order_id, Price: price, Size: size}
		filledBuyOrders = append(filledBuyOrders, buyOrder)
	}
	return filledBuyOrders
}

// 過去3日分の利益を取得する関数
func GetOKexResults() (string, error) {
	if database.CurrentDriver() == "postgres" {
		return getOKexResultsPostgres()
	}
	return getOKexResultsMySQL()
}

func getOKexResultsMySQL() (string, error) {
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
	return formatOKexResults(rows)
}

func getOKexResultsPostgres() (string, error) {
	rows, err := models.AppDB.Query(`
		select
			'total' as date,
			round((sum(average.profit) * 0.9988)::numeric, 4)::float as profit,
			count(average.profit) as count,
			round((avg(average.profit) * 0.9988)::numeric, 4)::float as ppt
		from
			(select
				TO_CHAR(updatetime, 'YYYY-MM-DD') as date,
				sum((sell_price - price) * size * 106) as profit
			from buy_orders
			where sell_order_state = 2
			group by TO_CHAR(updatetime, 'YYYY-MM-DD')
		) average
		union all
		select
			TO_CHAR(updatetime, 'YYYY-MM-DD') as date,
			round((sum((sell_price - price) * size * 106) * 0.9988)::numeric, 4)::float as profit,
			count(sell_price) as count,
			round((sum((sell_price - price) * size * 106) / count(sell_price) * 0.9988)::numeric, 4)::float as ppt
		from buy_orders
		where sell_order_state = 2
		group by TO_CHAR(updatetime, 'YYYY-MM-DD')
		order by date desc
		limit 4
		`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	return formatOKexResults(rows)
}

type okexRowScanner interface {
	Next() bool
	Scan(...any) error
	Err() error
}

func formatOKexResults(rows okexRowScanner) (string, error) {
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
	if err := rows.Err(); err != nil {
		return "", err
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

func GetOKJCancelledOrders() ([]OkjBuyOrder, error) {
	okjBuyOrders := []OkjBuyOrder{}
	if err := models.GormDB.Limit(100).Where("state = ?", 0).Find(&okjBuyOrders).Error; err != nil {
		return nil, errors.New("failed to do GetCancelledBuyOrders")
	}
	return okjBuyOrders, nil
}

func UpdateCancelledOrder(order_id string) error {
	var query string
	if database.CurrentDriver() == "postgres" {
		query = `update ` + TableName + ` set state = -1, sell_order_state = -1 where order_id = $1`
	} else {
		query = `update ` + TableName + ` set state = -1, sell_order_state = -1 where order_id = ?`
	}
	_, err := models.AppDB.Exec(query, order_id)
	if err != nil {
		return err
	}
	return nil
}
