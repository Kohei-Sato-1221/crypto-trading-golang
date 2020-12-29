package models

import (
	"errors"
	"log"
	"strconv"
	"strings"
	"time"
)

type OrderEvent struct {
	OrderId     string    `json:"orderid"`
	Time        time.Time `json:"time"`
	ProductCode string    `json:"product_code"`
	Side        string    `json:"side"`
	Price       float64   `json:"price"`
	Size        float64   `json:"size"`
	Exchange    string    `json:"exchange"`
	Filled      int       `json:"filled"`
}

func (e *OrderEvent) BuyOrder() error {
	cmd1, _ := MysqlDbConn.Prepare("INSERT INTO buy_orders (orderid, product_code, side, price, size, exchange) VALUES (?, ?, ?, ?, ?, ?)")
	log.Printf("BuyOrder() orderid:%s price:%10.2f size:%s side:%s", e.OrderId, e.Price, e.Side, e.Size)
	_, err := cmd1.Exec(e.OrderId, e.ProductCode, e.Side, e.Price, e.Size, e.Exchange)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			log.Println(err)
			return nil
		}
		return errors.New("Error in BuyOrder()")
	}
	return nil
}

func (e *OrderEvent) SellOrder(pid string) error {
	cmd1, _ := MysqlDbConn.Prepare("INSERT INTO sell_orders (parentid, orderid, product_code, side, price, size, exchange) VALUES (?, ?, ?, ?, ?, ?, ?)")
	_, err := cmd1.Exec(pid, e.OrderId, e.ProductCode, e.Side, e.Price, e.Size, e.Exchange)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			log.Println(err)
			return nil
		}
		return errors.New("Error in SellOrder()")
	}
	return nil
}

func FilledCheck(productCode string) ([]string, error) {
	// cmd, _ := MysqlDbConn.Prepare(`SELECT orderid FROM buy_orders WHERE filled = 0 and orderid != '' and product_code = ?`)
	cmd, _ := MysqlDbConn.Prepare(`SELECT orderid FROM buy_orders WHERE filled = 0 and orderid != '' and product_code = ? union SELECT orderid FROM sell_orders WHERE filled = 0 and orderid != '' and product_code = ?`)
	rows, err := cmd.Query(productCode, productCode)
	if err != nil {
		log.Printf("Failure to exec query..... %v", err)
		return nil, err
	}

	var cnt int = 0
	var ids []string
	for rows.Next() {
		var orderId string

		if err := rows.Scan(&orderId); err != nil {
			log.Printf("Failure to get records..... %v", err)
			log.Println("Failure to get records.....")
			return nil, err
		}
		cnt++
		ids = append(ids, orderId)
	}
	return ids, nil
}

func DeleteStrangeBuyOrderRecords() int {
	cmd := `DELETE FROM buy_orders WHERE orderid = ''`
	MysqlDbConn.Query(cmd)
	cnt := 0
	//	for rows.Next(){
	//		rows.Scan(&cnt)
	//	}
	return cnt
}

func DetermineCancelledOrder(max_buy_orders int, noNeedToCancal string) string {
	cmd1, _ := MysqlDbConn.Prepare(`SELECT CASE WHEN MAX(t1.c1) < ? THEN ? ELSE MAX(t2.c2) END FROM (SELECT COUNT(orderid) c1 FROM buy_orders WHERE filled = 0 and orderid != '') t1, (SELECT orderid c2 FROM buy_orders WHERE filled = 0 and orderid != '' ORDER BY price ASC LIMIT 1) t2)`)
	var buy_orders_limit int = max_buy_orders
	if max_buy_orders > 8 {
		buy_orders_limit = 8
	}
	rows, err := cmd1.Query(buy_orders_limit, noNeedToCancal)
	if err != nil {
		return noNeedToCancal
	}
	defer rows.Close()

	var orderid string
	for rows.Next() {
		if err := rows.Scan(&orderid); err != nil {
			log.Println("Failure to get record.....")
			return noNeedToCancal
		}
	}
	if orderid == "" {
		return noNeedToCancal
	}
	return orderid
}

/*
 * 注文前の判断メソッド
 * 買り注文の前に呼ばれ、
 　 1. 未約定の買い注文数 < 最大買い注文数
 　 2. 未約定の売り注文数 < 最大売り注文数
   を両方満たす場合にtrueを返却。
*/
func ShouldPlaceBuyOrder(max_buy_orders, max_sell_orders int) bool {
	rows, err := MysqlDbConn.Query(`SELECT COUNT(orderid) FROM buy_orders WHERE filled = 0 and orderid != '' union all SELECT COUNT(orderid) FROM sell_orders WHERE filled = 0 and orderid != ''`)
	if err != nil {
		return true
	}
	defer rows.Close()

	var cnt int
	rowCnt := 0
	numberOfExistingBuyOrders := 0
	numberOfExistingSellOrders := 0
	for rows.Next() {
		if err := rows.Scan(&cnt); err != nil {
			log.Println("Failure to get records.....")
			return true
		}
		if rowCnt == 0 {
			numberOfExistingBuyOrders = cnt
		}
		if rowCnt == 0 {
			numberOfExistingSellOrders = cnt
		}
		rowCnt = rowCnt + 1
	}
	if numberOfExistingBuyOrders < max_buy_orders &&
		numberOfExistingSellOrders < max_sell_orders {
		return false
	}
	return true
}

type Idprice struct {
	OrderId     string  `json:"orderid"`
	Price       float64 `json:"price"`
	ProductCode string  `json:"product_code"`
	Size        float64 `json:"size"`
	Exchange    string  `json:"exchange"`
}

func FilledCheckWithSellOrder() []Idprice {
	rows, err := MysqlDbConn.Query(`SELECT orderid, price, product_code, size, exchange FROM buy_orders WHERE filled = 1 and orderid != ''`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var cnt int = 0
	var idprices []Idprice
	for rows.Next() {
		var orderId string
		var price float64
		var product_code string
		var size float64
		var exchange string

		if err := rows.Scan(&orderId, &price, &product_code, &size, &exchange); err != nil {
			log.Println("Failure to get records.....")
			return nil
		}
		cnt++
		idprice := Idprice{OrderId: orderId, Price: price, ProductCode: product_code, Size: size, Exchange: exchange}
		idprices = append(idprices, idprice)
	}
	return idprices
}

func UpdateFilledOrder(orderId string) error {
	cmd1, _ := MysqlDbConn.Prepare(`update buy_orders set filled = 1 where orderid = ?`)
	_, err := cmd1.Exec(orderId)
	if err != nil {
		return err
	}
	cmd2, _ := MysqlDbConn.Prepare(`update sell_orders set filled = 1 where orderid = ?`)
	_, err = cmd2.Exec(orderId)
	if err != nil {
		return err
	}
	return nil
}

func UpdateCancelledOrder(orderId string) error {
	cmd1, _ := MysqlDbConn.Prepare(`update buy_orders set filled = -1 where orderid = ?`)
	_, err := cmd1.Exec(orderId)
	if err != nil {
		return err
	}
	cmd2, _ := MysqlDbConn.Prepare(`update sell_orders set filled = -1 where orderid = ?`)
	_, err = cmd2.Exec(orderId)
	if err != nil {
		return err
	}
	return nil
}

func UpdateFilledOrderWithBuyOrder(orderId string) error {
	log.Printf("##")
	log.Printf("##UpdateFilledOrderWithBuyOrder: %v", orderId)
	log.Printf("##")
	cmd1, _ := MysqlDbConn.Prepare(`update buy_orders set filled = 2 where orderid = ?`)
	_, err := cmd1.Exec(orderId)
	if err != nil {
		return err
	}
	return nil
}

func SyncBuyOrders(events *[]OrderEvent) {
	for _, e := range *events {
		cmd1, _ := MysqlDbConn.Prepare(`SELECT COUNT(*) FROM buy_orders WHERE orderid = ?`)
		defer cmd1.Close()
		rowsExist, _ := cmd1.Query(e.OrderId)
		cnt := 0
		for rowsExist.Next() {
			rowsExist.Scan(&cnt)
		}
		defer rowsExist.Close()
		if cnt == 0 {
			state := "INSERT INTO buy_orders (orderid, product_code, side, price, size, exchange, filled) VALUES (" +
				"'" + e.OrderId + "'," +
				"'" + e.ProductCode + "'," +
				"'" + e.Side + "'," +
				"'" + strconv.FormatFloat(e.Price, 'f', 4, 64) + "'," +
				"'" + strconv.FormatFloat(e.Size, 'f', 4, 64) + "'," +
				"'" + e.Exchange + "'," +
				"'" + strconv.Itoa(e.Filled) + "')"
			log.Printf("state: %v", state)
			_, err := MysqlDbConn.Exec(state)
			if err != nil {
				log.Println("Failure to do SyncBuyOrders..... %v", err)
			} else {
				log.Printf("orderid %v has been newly inserted!", e.OrderId)
			}
		}
		rowsExist.Close()
	}
}
