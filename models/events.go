package models

import (
	"errors"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/enums"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/utils"
	"log"
	"strconv"
	"strings"
	"time"
)

type OrderEvent struct {
	OrderID     string    `json:"order_id"`
	Time        time.Time `json:"time"`
	ProductCode string    `json:"product_code"`
	Side        string    `json:"side"`
	Price       float64   `json:"price"`
	Size        float64   `json:"size"`
	Exchange    string    `json:"exchange"`
	Filled      int       `json:"filled"`
	Strategy    int       `json:"strategy"`
}

//TODO structを整理すること
//TODO timestampはstring以外の型にすること
type BuyOrder struct {
	ID          uint    `gorm:"primary_key"`
	OrderID     string  `json:"order_id"`
	ProductCode string  `json:"product_code"`
	Side        string  `json:"side"`
	Price       float64 `json:"price"`
	Size        float64 `json:"size"`
	Exchange    string  `json:"exchange"`
	Filled      int     `json:"filled"`
	Timestamp   string  `json:"timestamp"`
	Updatetime  string  `json:"updatetime"`
}

func (e *OrderEvent) BuyOrder() error {
	cmd1, err := AppDB.Prepare("INSERT INTO buy_orders (order_id, product_code, side, price, size, exchange, strategy) VALUES (?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		log.Printf("[ERROR] BuyOrder01:%s\n", err)
		return err
	}
	log.Printf("BuyOrder() order_id:%s price:%10.2f size:%s side:%s strategy:%s", e.OrderID, e.Price, e.Side, e.Size, e.Strategy)
	_, err = cmd1.Exec(e.OrderID, e.ProductCode, e.Side, e.Price, e.Size, e.Exchange, e.Strategy)
	if err != nil {
		log.Printf("[ERROR] BuyOrder02:%s\n", err)
		return err
	}
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			log.Printf("[ERROR] BuyOrder03:%s\n", err)
			return nil
		}
		return errors.New("Error in BuyOrder()")
	}
	return nil
}

func (e *OrderEvent) SellOrder(pid string) error {
	cmd1, _ := AppDB.Prepare("INSERT INTO sell_orders (parentid, order_id, product_code, side, price, size, exchange) VALUES (?, ?, ?, ?, ?, ?, ?)")
	_, err := cmd1.Exec(pid, e.OrderID, e.ProductCode, e.Side, e.Price, e.Size, e.Exchange)
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
	cmd, _ := AppDB.Prepare(`SELECT order_id FROM buy_orders WHERE filled = 0 and order_id != '' and product_code = ? union SELECT order_id FROM sell_orders WHERE filled = 0 and order_id != '' and product_code = ?`)
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
	cmd := `DELETE FROM buy_orders WHERE order_id = ''`
	AppDB.Query(cmd)
	cnt := 0
	return cnt
}

func GetCancelledBuyOrders() ([]BuyOrder, error) {
	buyOrders := []BuyOrder{}
	if err := GormDB.Limit(100).Where("filled = ?", 0).Find(&buyOrders).Error; err != nil {
		return nil, errors.New("failed to do GetCancelledBuyOrders")
	}
	return buyOrders, nil
}

/*
 * 注文前の判断メソッド
 * 買り注文の前に呼ばれ、
 　 1. 未約定の買い注文数 < 最大買い注文数
 　 2. 未約定の売り注文数 < 最大売り注文数
   を両方満たす場合にtrueを返却。
*/
func ShouldPlaceBuyOrder(max_buy_orders, max_sell_orders int) (bool, error) {
	rows, err := AppDB.Query(`SELECT COUNT(order_id) FROM buy_orders WHERE filled = 0 and order_id != '' union all SELECT COUNT(order_id) FROM sell_orders WHERE filled = 0 and order_id != ''`)
	if err != nil {
		return true, err
	}
	defer rows.Close()

	var cnt int
	rowCnt := 0
	numberOfExistingBuyOrders := 0
	numberOfExistingSellOrders := 0
	for rows.Next() {
		if err := rows.Scan(&cnt); err != nil {
			log.Println("Failure to get records.....")
			return true, err
		}
		if rowCnt == 0 {
			numberOfExistingBuyOrders = cnt
		}
		if rowCnt == 0 {
			numberOfExistingSellOrders = cnt
		}
		rowCnt = rowCnt + 1
	}
	log.Printf("ShouldPlaceBuyOrder: numberOfExistingBuyOrders:%v numberOfExistingSellOrders:%v", numberOfExistingBuyOrders, numberOfExistingSellOrders)
	if numberOfExistingBuyOrders < max_buy_orders &&
		numberOfExistingSellOrders < max_sell_orders {
		return false, nil
	}
	return true, nil
}

type BuyOrderInfo struct {
	OrderID     string  `json:"order_id"`
	Price       float64 `json:"price"`
	ProductCode string  `json:"product_code"`
	Size        float64 `json:"size"`
	Exchange    string  `json:"exchange"`
	Strategy    float64 `json:"strategy"`
}

func (buyOrderInfo *BuyOrderInfo) CalculateSellOrderPrice() float64 {
	if buyOrderInfo.Strategy == enums.Stg3BtcLtp90 ||
		buyOrderInfo.Strategy == enums.Stg14EthLtp90 {
		return utils.Round(buyOrderInfo.Price * 1.03)
	} else {
		return utils.Round(buyOrderInfo.Price * 1.015)
	}
}

func CalculateMinuteToExpire(strategy int) int {
	if strategy == enums.Stg3BtcLtp90 ||
		strategy == enums.Stg14EthLtp90 {
		return 1440 //1day
	} else {
		return 3600 //2.5days
	}
}

func CheckFilledBuyOrders() []BuyOrderInfo {
	rows, err := AppDB.Query(`SELECT order_id, price, product_code, size, exchange, strategy FROM buy_orders WHERE filled = 1 and order_id != ''`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var cnt int = 0
	var buyOrderInfos []BuyOrderInfo
	for rows.Next() {
		var order_id string
		var price float64
		var product_code string
		var size float64
		var exchange string
		var strategy int

		if err := rows.Scan(&order_id, &price, &product_code, &size, &exchange, &strategy); err != nil {
			log.Println("Failure to get records.....")
			return nil
		}
		cnt++
		buyOrderInfo := BuyOrderInfo{OrderID: order_id, Price: price, ProductCode: product_code, Size: size, Exchange: exchange}
		buyOrderInfos = append(buyOrderInfos, buyOrderInfo)
	}
	return buyOrderInfos
}

func UpdateFilledOrder(order_id string) error {
	cmd1, _ := AppDB.Prepare(`update buy_orders set filled = 1 where order_id = ?`)
	_, err := cmd1.Exec(order_id)
	if err != nil {
		return err
	}
	cmd2, _ := AppDB.Prepare(`update sell_orders set filled = 1 where order_id = ?`)
	_, err = cmd2.Exec(order_id)
	if err != nil {
		return err
	}
	return nil
}

func UpdateCancelledBuyOrder(order_id string) error {
	cmd, _ := AppDB.Prepare(`update buy_orders set filled = -1 where order_id = ?`)
	_, err := cmd.Exec(order_id)
	if err != nil {
		return err
	}
	return nil
}

func UpdateFilledOrderWithBuyOrder(order_id string) error {
	log.Printf("##")
	log.Printf("##UpdateFilledOrderWithBuyOrder: %v", order_id)
	log.Printf("##")
	cmd1, _ := AppDB.Prepare(`update buy_orders set filled = 2 where order_id = ?`)
	_, err := cmd1.Exec(order_id)
	if err != nil {
		return err
	}
	return nil
}

func SyncBuyOrders(events *[]OrderEvent) {
	for _, e := range *events {
		cmd1, _ := AppDB.Prepare(`SELECT COUNT(*) FROM buy_orders WHERE order_id = ?`)
		defer cmd1.Close()
		rowsExist, _ := cmd1.Query(e.OrderID)
		cnt := 0
		for rowsExist.Next() {
			rowsExist.Scan(&cnt)
		}
		defer rowsExist.Close()
		if cnt == 0 {
			state := "INSERT INTO buy_orders (order_id, product_code, side, price, size, exchange, filled) VALUES (" +
				"'" + e.OrderID + "'," +
				"'" + e.ProductCode + "'," +
				"'" + e.Side + "'," +
				"'" + strconv.FormatFloat(e.Price, 'f', 4, 64) + "'," +
				"'" + strconv.FormatFloat(e.Size, 'f', 4, 64) + "'," +
				"'" + e.Exchange + "'," +
				"'" + strconv.Itoa(e.Filled) + "')"
			log.Printf("state: %v", state)
			_, err := AppDB.Exec(state)
			if err != nil {
				log.Println("Failure to do SyncBuyOrders..... %v", err)
			} else {
				log.Printf("order_id %v has been newly inserted!", e.OrderID)
			}
		}
		rowsExist.Close()
	}
}

//過去3日分の利益を取得する関数
func GetResults() (string, error) {
	rows, err := AppDB.Query(`
		select 
		 'Total' date,
		 round(sum(average.profit) * 0.9989, 2) profit,
		 count(average.profit) count,
		 round(avg(average.profit) * 0.9989, 2) ppt
		from
		(select 
			DATE_FORMAT(a.updatetime, '%Y-%m-%d') date,
			sum((a.price * a.size) - (b.price * b.size)) profit
		from 
			sell_orders a,
			buy_orders b
		where 
			a.parentid = b.order_id and a.filled = 1
			and DATE_FORMAT(a.updatetime, '%Y-%m-%d') <> '2020-01-01'
		group by date) average
		
		union
		
		select 
		 result.date date,
		 round(sum(result.profit) * 0.9989, 2) profit,
		 count(result.profit) count,
		 round(sum(result.profit) / count(result.profit) * 0.9989, 2) ppt
		from
		(select 
			DATE_FORMAT(a.updatetime, '%Y-%m-%d') date,
			(a.price * a.size) - (b.price * b.size) profit
		from 
			sell_orders a,
			buy_orders b
		where a.parentid = b.order_id and a.filled = 1
		order by date desc)
		result
		group by date
		order by date desc
		limit 4
		`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var sb strings.Builder
	sb.WriteString("【bitflyer 自動売買 収益】\n")
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
