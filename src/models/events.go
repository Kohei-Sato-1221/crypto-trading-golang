package models

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
	"config"
	"errors"
)

type OrderEvent struct {
	OrderId     string    `json:"orderid"`
	Time        time.Time `json:"time"`
	ProductCode string    `json:"product_code"`
	Side        string    `json:"side"`
	Price       float64   `json:"price"`
	Size        float64   `json:"size"`
	Exchange    string    `json:"exchange"`
}


func (e *OrderEvent) BuyOrder() error {
	cmd := fmt.Sprintf("INSERT INTO buy_orders (orderid, time, product_code, side, price, size, exchange) VALUES (?, ?, ?, ?, ?, ?, ?)")
	_, err := DbConnection.Exec(cmd, e.OrderId, e.Time.Format(time.RFC3339), e.ProductCode, e.Side, e.Price, e.Size, e.Exchange)
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
	cmd := fmt.Sprintf("INSERT INTO sell_orders (parentid, orderid, time, product_code, side, price, size, exchange) VALUES (?, ?, ?, ?, ?, ?, ?, ?)")
	_, err := DbConnection.Exec(cmd, pid, e.OrderId, e.Time.Format(time.RFC3339), e.ProductCode, e.Side, e.Price, e.Size, e.Exchange)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			log.Println(err)
			return nil
		}
		return errors.New("Error in SellOrder()")
	}
	return nil
}


func FilledCheck() ([]string, error){
	cmd := `SELECT orderid FROM buy_orders WHERE filled = 0 and orderid != '' union SELECT orderid FROM sell_orders WHERE filled = 0 and orderid != '';`
	rows, err := DbConnection.Query(cmd)
	if err != nil {
		log.Printf("Failure to exec query..... %v", err)
		return nil, err
	}
	defer rows.Close()

	var cnt int = 0
	var ids []string
	for rows.Next() {
		var orderId string
		
		if err := rows.Scan(&orderId); err != nil {
			log.Printf("Failure to get records..... %v", err)
			log.Fatal("Failure to get records.....")
			return nil, err
		}
		cnt++
		ids = append(ids, orderId)
	}
	return ids, nil
}

func CountUnfilledBuyOrders() int{
	cmd := `SELECT COUNT(orderid) FROM buy_orders WHERE filled = 0 and orderid != '';`
	rows, err := DbConnection.Query(cmd)
	if err != nil {
		return 999
	}
	defer rows.Close()

	var cnt int
	for rows.Next() {
		if err := rows.Scan(&cnt); err != nil {
			log.Fatal("Failure to get records.....")
			return 999
		}
		break
	}
	return cnt
}

type Idprice struct {
	OrderId     string    `json:"orderid"`
	Price       float64   `json:"price"`
}

func FilledCheckWithSellOrder() []Idprice{
	cmd := `SELECT orderid, price FROM buy_orders WHERE filled = 1 and orderid != '';`
	rows, err := DbConnection.Query(cmd)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var cnt int = 0
	var idprices []Idprice;
	for rows.Next() {
		var orderId string
		var price float64
		
		if err := rows.Scan(&orderId, &price); err != nil {
			log.Fatal("Failure to get records.....")
			return nil
		}
		cnt++
		idprice := Idprice{OrderId: orderId, Price: price}
		idprices = append(idprices, idprice)
	}
	return idprices
}

func UpdateFilledOrder(orderId string) error{
	cmd := fmt.Sprintf("update buy_orders set filled = 1 where orderid = ?")
	_, err := DbConnection.Exec(cmd, orderId)
	if err != nil {
		return err
	}
	cmd = fmt.Sprintf("update sell_orders set filled = 1 where orderid = ?")
	_, err = DbConnection.Exec(cmd, orderId)
	if err != nil {
		return err
	}
	return nil
}

func UpdateFilledOrderWithBuyOrder(orderId string) error{
	cmd := fmt.Sprintf("update buy_orders set filled = 2 where orderid = ?")
	_, err := DbConnection.Exec(cmd, orderId)
	if err != nil {
		return err
	}
	return nil
}

func UpdateFilledSellOrder(orderId string) error{
	cmd := fmt.Sprintf("update sell_orders set filled = 1 where orderid = ?")
	_, err := DbConnection.Exec(cmd, orderId)
	if err != nil {
		return err
	}
	return nil
}


type SignalEvent struct {
	Time        time.Time `json:"time"`
	ProductCode string    `json:"product_code"`
	Side        string    `json:"side"`
	Price       float64   `json:"price"`
	Size        float64   `json:"size"`
}


func (s *SignalEvent) Save() bool {
	cmd := fmt.Sprintf("INSERT INTO %s (time, product_code, side, price, size) VALUES (?, ?, ?, ?, ?)", tableNameSignalEvents)
	_, err := DbConnection.Exec(cmd, s.Time.Format(time.RFC3339), s.ProductCode, s.Side, s.Price, s.Size)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			log.Println(err)
			return true
		}
		return false
	}
	return true
}

// BUY or SELL
type SignalEvents struct {
	Signals []SignalEvent `json:"signals,omitempty"`
}

func NewSignalEvents() *SignalEvents {
	return &SignalEvents{}
}

// DBにイベント情報があったら最新のものを取得して返却
func GetSignalEventsByCount(loadEvents int) *SignalEvents {
	cmd := fmt.Sprintf(`SELECT * FROM (
        SELECT time, product_code, side, price, size FROM %s WHERE product_code = ? ORDER BY time DESC LIMIT ? )
        ORDER BY time ASC;`, tableNameSignalEvents)
	rows, err := DbConnection.Query(cmd, config.Config.ProductCode, loadEvents)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var signalEvents SignalEvents
	for rows.Next() {
		var signalEvent SignalEvent
		rows.Scan(&signalEvent.Time, &signalEvent.ProductCode, &signalEvent.Side, &signalEvent.Price, &signalEvent.Size)
		signalEvents.Signals = append(signalEvents.Signals, signalEvent)
	}
	err = rows.Err()
	if err != nil {
		return nil
	}
	return &signalEvents
}

// 指定時間以降のイベントを取得する
func GetSignalEventsAfterTime(timeTime time.Time) *SignalEvents {
	cmd := fmt.Sprintf(`SELECT * FROM (
                SELECT time, product_code, side, price, size FROM %s
                WHERE DATETIME(time) >= DATETIME(?)
                ORDER BY time DESC
            ) ORDER BY time ASC;`, tableNameSignalEvents)
	rows, err := DbConnection.Query(cmd, timeTime.Format(time.RFC3339))
	if err != nil {
		return nil
	}
	defer rows.Close()

	var signalEvents SignalEvents
	for rows.Next() {
		var signalEvent SignalEvent
		rows.Scan(&signalEvent.Time, &signalEvent.ProductCode, &signalEvent.Side, &signalEvent.Price, &signalEvent.Size)
		signalEvents.Signals = append(signalEvents.Signals, signalEvent)
	}
	return &signalEvents
}

func (s *SignalEvents) CanBuy() bool {
	return true
}

func (s *SignalEvents) CanSell() bool {
	return true
}

func (s *SignalEvents) Sugar() bool {
	return true
}


func (s *SignalEvents) Buy(ProductCode string, time time.Time, price, size float64, save bool) bool {
	if !s.CanBuy() {
		return false
	}
	signalEvent := SignalEvent{
		ProductCode: ProductCode,
		Time:        time,
		Side:        "BUY",
		Price:       price,
		Size:        size,
	}
	if save {
		signalEvent.Save()
	}
	s.Signals = append(s.Signals, signalEvent)
	return true
}

func (s *SignalEvents) Sell(productCode string, time time.Time, price, size float64, save bool) bool {
	if !s.CanSell() {
		return false
	}
	signalEvent := SignalEvent{
		ProductCode: productCode,
		Time:        time,
		Side:        "SELL",
		Price:       price,
		Size:        size,
	}
	if save {
		signalEvent.Save()
	}
	s.Signals = append(s.Signals, signalEvent)
	return true
}

func (s *SignalEvents) Profit() float64 {
	total := 0.0
	beforeSell := 0.0
	isHolding := false
	for i, signalEvent := range s.Signals {
		if i == 0 && signalEvent.Side == "SELL" {
			continue
		}
		if signalEvent.Side == "BUY" {
			total -= signalEvent.Price * signalEvent.Size
			isHolding = true
		}
		if signalEvent.Side == "SEL" {
			total += signalEvent.Price * signalEvent.Size
			isHolding = false
			beforeSell = total
		}
	}
	if isHolding == true {
		return beforeSell
	}
	return total
}

func (s SignalEvents) MarshalJSON() ([]byte, error) {
	value, err := json.Marshal(&struct {
		Signals []SignalEvent `json:"signals,omitempty"`
		Profit  float64       `json:"profit,omitempty"`
	}{
		Signals: s.Signals,
		Profit:  s.Profit(),
	})
	if err != nil {
		return nil, err
	}
	return value, err
}

func (s *SignalEvents) CollectAfter(time time.Time) *SignalEvents {
	for i, signal := range s.Signals {
		if time.After(signal.Time) {
			continue
		}
		return &SignalEvents{Signals: s.Signals[i:]}
	}
	return nil
}
