package models

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/config"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/database"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/enums"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/utils"
)

const (
	OrderStatusUnfilled              = "UNFILLED"
	OrderStatusFilled                = "FILLED"
	OrderStatusFilledSellOrderPlaced = "FILLED(SELL ORDER PLACED)"
	OrderStatusCancelled             = "CANCELLED"
)

// remarks に記録する既定文言。
const (
	RemarkPlacedByApp    = "placed by trading app" // ボットが発注した注文
	RemarkPlacedManually = "placed manually"       // 取引所で手動発注され syncBuyOrders が取り込んだ注文
)

// formatExpireDate はログ出力用に expire_date(UTC) を文字列化する。未設定なら "nil"。
func formatExpireDate(expireDate *time.Time) string {
	if expireDate == nil {
		return "nil"
	}
	return expireDate.UTC().Format(time.RFC3339)
}

type OrderEvent struct {
	OrderID     string     `json:"order_id"`
	Time        time.Time  `json:"time"`
	ProductCode string     `json:"product_code"`
	Side        string     `json:"side"`
	Price       float64    `json:"price"`
	Size        float64    `json:"size"`
	Exchange    string     `json:"exchange"`
	Status      string     `json:"status"`
	Strategy    int        `json:"strategy"`
	ExpireDate  *time.Time `json:"expire_date"` // 注文の有効期限(UTC)。nilならDBにはNULLを保存する
	Remarks     string     `json:"remarks"`     // 空なら既定文言を使う
}

/*
expireDateValue はDBへ渡す expire_date の値を返す。

必ずUTCへ正規化し、未設定(nil)の場合はNULLとして保存されるよう nil を返す。
DB上の expire_date は常にUTCであることを前提に、失効判定・ローリング判定を行う。
*/
func (e *OrderEvent) expireDateValue() any {
	if e.ExpireDate == nil {
		return nil
	}
	return e.ExpireDate.UTC()
}

// TODO structを整理すること
// TODO timestampはstring以外の型にすること
type BuyOrder struct {
	ID          uint    `gorm:"primary_key"`
	OrderID     string  `json:"order_id"`
	ProductCode string  `json:"product_code"`
	Side        string  `json:"side"`
	Price       float64 `json:"price"`
	Size        float64 `json:"size"`
	Exchange    string  `json:"exchange"`
	Status      string  `json:"status"`
	Timestamp   string  `json:"timestamp"`
	Updatetime  string  `json:"updatetime"`
}

func (e *OrderEvent) BuyOrder() error {
	var query string
	if database.CurrentDriver() == "postgres" {
		query = "INSERT INTO buy_orders (order_id, product_code, side, price, size, exchange, strategy, remarks, expire_date) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)"
	} else {
		query = "INSERT INTO buy_orders (order_id, product_code, side, price, size, exchange, strategy, remarks, expire_date) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)"
	}
	remarks := e.Remarks
	if remarks == "" {
		remarks = RemarkPlacedByApp
	}
	log.Printf("BuyOrder() order_id:%s price:%10.2f size:%f side:%s strategy:%d expire_date(UTC):%s",
		e.OrderID, e.Price, e.Size, e.Side, e.Strategy, formatExpireDate(e.ExpireDate))
	_, err := AppDB.Exec(query, e.OrderID, e.ProductCode, e.Side, e.Price, e.Size, e.Exchange, e.Strategy, remarks, e.expireDateValue())
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "duplicate key") {
			log.Printf("[ERROR] BuyOrder duplicate:%s\n", err)
			return nil
		}
		log.Printf("[ERROR] BuyOrder:%s\n", err)
		return err
	}
	return nil
}

/*
SellOrder は sell_orders へINSERTする。

OrderEvent の Remarks / ExpireDate をそのまま使う後方互換のシグネチャ。
明示的に remarks / expire_date を指定したい場合は SellOrderWithMeta を使う。
*/
func (e *OrderEvent) SellOrder(pid string) error {
	return e.SellOrderWithMeta(pid, e.Remarks, e.ExpireDate)
}

/*
SellOrderWithMeta は remarks / expire_date を明示して sell_orders へINSERTする。

expireDate は必ずUTCへ正規化して保存する（nilならNULL）。
remarks が空文字の場合はNULLを保存し、既存の挙動を変えない。
*/
func (e *OrderEvent) SellOrderWithMeta(pid, remarks string, expireDate *time.Time) error {
	var query string
	if database.CurrentDriver() == "postgres" {
		query = "INSERT INTO sell_orders (parentid, order_id, product_code, side, price, size, exchange, remarks, expire_date) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)"
	} else {
		query = "INSERT INTO sell_orders (parentid, order_id, product_code, side, price, size, exchange, remarks, expire_date) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)"
	}

	var remarksValue any
	if remarks != "" {
		remarksValue = remarks
	}
	var expireDateValue any
	if expireDate != nil {
		expireDateValue = expireDate.UTC()
	}

	log.Printf("SellOrder() order_id:%s parentid:%s price:%10.2f size:%f expire_date(UTC):%s",
		e.OrderID, pid, e.Price, e.Size, formatExpireDate(expireDate))
	_, err := AppDB.Exec(query, pid, e.OrderID, e.ProductCode, e.Side, e.Price, e.Size, e.Exchange, remarksValue, expireDateValue)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "duplicate key") {
			log.Println(err)
			return nil
		}
		return fmt.Errorf("failed to insert sell order (order_id:%s parentid:%s price:%.2f size:%v): %w",
			e.OrderID, pid, e.Price, e.Size, err)
	}
	return nil
}

func FilledCheck(productCode string) ([]string, error) {
	var query string
	if database.CurrentDriver() == "postgres" {
		query = `SELECT order_id FROM buy_orders WHERE status = 'UNFILLED' and order_id != '' and product_code = $1 union SELECT order_id FROM sell_orders WHERE status = 'UNFILLED' and order_id != '' and product_code = $2`
	} else {
		query = `SELECT order_id FROM buy_orders WHERE status = 'UNFILLED' and order_id != '' and product_code = ? union SELECT order_id FROM sell_orders WHERE status = 'UNFILLED' and order_id != '' and product_code = ?`
	}
	rows, err := AppDB.Query(query, productCode, productCode)
	if err != nil {
		log.Printf("Failure to exec query..... %v", err)
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var orderId string
		if err := rows.Scan(&orderId); err != nil {
			log.Printf("Failure to get records..... %v", err)
			return nil, err
		}
		ids = append(ids, orderId)
	}
	return ids, nil
}

func DeleteStrangeBuyOrderRecords() int {
	_, _ = AppDB.Exec(`DELETE FROM buy_orders WHERE order_id = ''`)
	return 0
}

func GetUnfilledBuyOrders() ([]BuyOrder, error) {
	buyOrders := []BuyOrder{}
	if err := GormDB.Limit(100).Where("status = ?", OrderStatusUnfilled).Find(&buyOrders).Error; err != nil {
		return nil, errors.New("failed to do GetUnfilledBuyOrders")
	}
	return buyOrders, nil
}

// BuyOrderSlotStatus は買い注文スロットの充足状況を表す。
type BuyOrderSlotStatus struct {
	UnfilledBuyCount  int
	UnfilledSellCount int
	MaxBuyOrders      int
	MaxSellOrders     int
	ShouldSkip        bool   // buy側が上限に達している場合のみtrue（買い注文をブロックする）
	SellWarning       bool   // sell側が上限に達している場合true（買い注文はブロックしない）
	Message           string // "未約定buy:n/max 未約定sell:m/max" 形式
}

/*
GetBuyOrderSlotStatus はスロット状況を返す。買い注文の前に呼ばれる。

【仕様】
  - buy側の上限超過 → ShouldSkip=true。買い注文をブロックする（JPY拘束の歯止め）
  - sell側の上限超過 → SellWarning=true。ShouldSkipには影響しない。
    Slackに警告を出したうえで発注は続行する
  - 現物の積み上がりに対する最終的な歯止めはbudget_criteria（JPY残高の下限）であり、
    その値の管理はユーザー責務。
*/
func GetBuyOrderSlotStatus(max_buy_orders, max_sell_orders int) (*BuyOrderSlotStatus, error) {
	rows, err := AppDB.Query(`SELECT COUNT(order_id) FROM buy_orders WHERE status = 'UNFILLED' and order_id != '' union all SELECT COUNT(order_id) FROM sell_orders WHERE status = 'UNFILLED' and order_id != ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cnt int
	rowCnt := 0
	numberOfExistingBuyOrders := 0
	numberOfExistingSellOrders := 0
	for rows.Next() {
		if err := rows.Scan(&cnt); err != nil {
			log.Println("Failure to get records.....")
			return nil, err
		}
		if rowCnt == 0 {
			numberOfExistingBuyOrders = cnt
		}
		if rowCnt == 1 {
			numberOfExistingSellOrders = cnt
		}
		rowCnt = rowCnt + 1
	}

	status := &BuyOrderSlotStatus{
		UnfilledBuyCount:  numberOfExistingBuyOrders,
		UnfilledSellCount: numberOfExistingSellOrders,
		MaxBuyOrders:      max_buy_orders,
		MaxSellOrders:     max_sell_orders,
		ShouldSkip:        numberOfExistingBuyOrders >= max_buy_orders,
		SellWarning:       numberOfExistingSellOrders >= max_sell_orders,
	}
	status.Message = fmt.Sprintf("未約定buy:%v/%v 未約定sell:%v/%v",
		numberOfExistingBuyOrders, max_buy_orders, numberOfExistingSellOrders, max_sell_orders)
	log.Println(status.Message)
	return status, nil
}

/*
ShouldPlaceBuyOrder はGetBuyOrderSlotStatus()への後方互換ラッパ。
第1返り値には ShouldSkip（= buy側の超過のみ）を返す。
sell側の超過は発注をブロックしないため、この返り値には影響しない。
*/
func ShouldPlaceBuyOrder(max_buy_orders, max_sell_orders int) (bool, error, string) {
	status, err := GetBuyOrderSlotStatus(max_buy_orders, max_sell_orders)
	if err != nil {
		return true, err, ""
	}
	return status.ShouldSkip, nil, status.Message
}

type BuyOrderInfo struct {
	OrderID     string  `json:"order_id"`
	Price       float64 `json:"price"`
	ProductCode string  `json:"product_code"`
	Size        float64 `json:"size"`
	Exchange    string  `json:"exchange"`
	Strategy    int     `json:"strategy"`
}

/*
SellProfitRate は買い戦略に対応する利確率を返す。
第2返り値はマッピング(enums.SellProfitRate)に定義があったかを示し、
false の場合は enums.SellProfitRateDefault(1.015)にフォールバックしている。
*/
func (buyOrderInfo *BuyOrderInfo) SellProfitRate() (float64, bool) {
	return enums.SellProfitRate(buyOrderInfo.Strategy)
}

/*
CalculateSellOrderPrice は売り指値(利確価格)を返す。
利確率は買い戦略ごとに変わる（買い指値が深い戦略ほど大きな利確幅を狙う）。
戦略値と利確率の対応は enums.sellProfitRates に集約している。
*/
func (buyOrderInfo *BuyOrderInfo) CalculateSellOrderPrice() float64 {
	rate, _ := buyOrderInfo.SellProfitRate()
	return utils.Round(buyOrderInfo.Price * rate)
}

/*
CalculateMinuteToExpire は買い注文の有効期限(分)を返す。
既定値は config.ini の [bitflyer] buy_minute_to_expire から読み込み、
未設定(0以下)の場合は config.DefaultBuyMinuteToExpire(10080分 = 7日)を使う。
*/
func CalculateMinuteToExpire(strategy int) int {
	if strategy == enums.Stg3BtcLtp90 ||
		strategy == enums.Stg14EthLtp90 {
		return 1440 // 1day
	}
	if config.Config.BFBuyMinuteToExpire > 0 {
		return config.Config.BFBuyMinuteToExpire
	}
	return config.DefaultBuyMinuteToExpire // 10080min = 7days
}

/*
CheckFilledBuyOrders は約定済みかつ売り注文がまだ無い買い注文を返す。

以前は失敗時に nil を返すだけだったため、呼び出し元(placeSellOrder)が
「売る対象なし」と解釈し、約定済み買い注文への売り注文発注が無言でスキップされていた。
DBの読み取りに失敗したことと対象が0件であることを呼び出し元が区別できるよう、
必ず error を返す（通知は app 層が行う）。
*/
func CheckFilledBuyOrders() ([]BuyOrderInfo, error) {
	rows, err := AppDB.Query(`SELECT order_id, price, product_code, size, exchange, strategy FROM buy_orders WHERE status = 'FILLED' and order_id != ''`)
	if err != nil {
		return nil, fmt.Errorf("CheckFilledBuyOrders: failed to query buy_orders: %w", err)
	}
	defer rows.Close()

	var buyOrderInfos []BuyOrderInfo
	for rows.Next() {
		var order_id string
		var price float64
		var product_code string
		var size float64
		var exchange string
		var strategy int

		if err := rows.Scan(&order_id, &price, &product_code, &size, &exchange, &strategy); err != nil {
			return nil, fmt.Errorf("CheckFilledBuyOrders: failed to scan buy_orders (scanned:%d): %w", len(buyOrderInfos), err)
		}
		// strategy はSELECTした値を必ず代入する（代入漏れがあると CalculateSellOrderPrice() の戦略分岐が働かない）
		buyOrderInfo := BuyOrderInfo{OrderID: order_id, Price: price, ProductCode: product_code, Size: size, Exchange: exchange, Strategy: strategy}
		buyOrderInfos = append(buyOrderInfos, buyOrderInfo)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("CheckFilledBuyOrders: failed to iterate buy_orders (scanned:%d): %w", len(buyOrderInfos), err)
	}
	return buyOrderInfos, nil
}

func UpdateFilledOrder(order_id string) error {
	var q1, q2 string
	if database.CurrentDriver() == "postgres" {
		q1 = `update buy_orders set status = 'FILLED' where order_id = $1`
		q2 = `update sell_orders set status = 'FILLED' where order_id = $1`
	} else {
		q1 = `update buy_orders set status = 'FILLED' where order_id = ?`
		q2 = `update sell_orders set status = 'FILLED' where order_id = ?`
	}
	_, err := AppDB.Exec(q1, order_id)
	if err != nil {
		return err
	}
	_, err = AppDB.Exec(q2, order_id)
	if err != nil {
		return err
	}
	return nil
}

func UpdateCancelledBuyOrder(order_id string) error {
	var query string
	if database.CurrentDriver() == "postgres" {
		query = `update buy_orders set status = 'CANCELLED' where order_id = $1`
	} else {
		query = `update buy_orders set status = 'CANCELLED' where order_id = ?`
	}
	_, err := AppDB.Exec(query, order_id)
	if err != nil {
		return err
	}
	return nil
}

func UpdateFilledOrderWithBuyOrder(order_id string) error {
	var query string
	if database.CurrentDriver() == "postgres" {
		query = `update buy_orders set status = $1 where order_id = $2`
	} else {
		query = `update buy_orders set status = ? where order_id = ?`
	}
	_, err := AppDB.Exec(query, OrderStatusFilledSellOrderPlaced, order_id)
	if err != nil {
		return err
	}
	return nil
}

/*
SyncBuyOrderFailure は SyncBuyOrders 内で発生した1件ぶんの失敗。

以前はログ出力のみだったため、取引所で発注済みの注文がDBに記録されないまま
（あるいは expire_date が入らないまま）無言で進んでいた。
Slack通知に必要なコンテキストを持たせ、通知は app 層(syncBuyOrders)が行う。
*/
type SyncBuyOrderFailure struct {
	Operation   string // "count" / "insert" / "expire_date"
	OrderID     string
	ProductCode string
	Side        string
	Price       float64
	Size        float64
	Strategy    int
	Err         error
}

// Error は failure 1件ぶんの説明文を返す（Slack通知の本文にそのまま載せる）。
func (f SyncBuyOrderFailure) Error() string {
	return fmt.Sprintf("op:%s OrderID:%s ProductCode:%s Side:%s Price:%.2f Size:%v Strategy:%v err:%v",
		f.Operation, f.OrderID, f.ProductCode, f.Side, f.Price, f.Size, f.Strategy, f.Err)
}

/*
SyncBuyOrders は取引所から取得した注文をDBへ取り込む。

1件の失敗で全体を止めないため、失敗しても後続のイベントの処理は継続し、
発生した失敗をまとめて返す。呼び出し元は返り値が空でなければSlackへ通知すること。
*/
func SyncBuyOrders(events *[]OrderEvent) []SyncBuyOrderFailure {
	var failures []SyncBuyOrderFailure
	var countQuery, insertQuery string
	if database.CurrentDriver() == "postgres" {
		countQuery = `SELECT COUNT(*) FROM buy_orders WHERE order_id = $1`
		insertQuery = `INSERT INTO buy_orders (order_id, product_code, side, price, size, exchange, status, remarks, strategy, expire_date) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`
	} else {
		countQuery = `SELECT COUNT(*) FROM buy_orders WHERE order_id = ?`
		insertQuery = `INSERT INTO buy_orders (order_id, product_code, side, price, size, exchange, status, remarks, strategy, expire_date) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	}

	for _, e := range *events {
		var cnt int
		err := AppDB.QueryRow(countQuery, e.OrderID).Scan(&cnt)
		if err != nil {
			log.Printf("[ERROR] SyncBuyOrders count query: %v", err)
			failures = append(failures, SyncBuyOrderFailure{
				Operation: "count", OrderID: e.OrderID, ProductCode: e.ProductCode, Side: e.Side,
				Price: e.Price, Size: e.Size, Strategy: e.Strategy, Err: err})
			continue
		}
		if cnt == 0 {
			status := e.Status
			if status == "" {
				status = OrderStatusUnfilled
			}
			// 取引所側で手動発注された注文を取り込むため、戦略値が未設定なら手動発注として記録する
			// （DBデフォルトの99「未記録」で埋まってしまうのを防ぐ）
			strategy := e.Strategy
			if strategy <= 0 {
				strategy = enums.StrategyManual
			}
			_, err := AppDB.Exec(insertQuery, e.OrderID, e.ProductCode, e.Side, e.Price, e.Size, e.Exchange, status, RemarkPlacedManually, strategy, e.expireDateValue())
			if err != nil {
				log.Printf("Failure to do SyncBuyOrders..... order_id:%v strategy:%v err:%v", e.OrderID, strategy, err)
				failures = append(failures, SyncBuyOrderFailure{
					Operation: "insert", OrderID: e.OrderID, ProductCode: e.ProductCode, Side: e.Side,
					Price: e.Price, Size: e.Size, Strategy: strategy, Err: err})
			} else {
				log.Printf("order_id %v has been newly inserted! strategy:%v expire_date(UTC):%s",
					e.OrderID, strategy, formatExpireDate(e.ExpireDate))
			}
			continue
		}
		// 既存レコードは取引所APIが返す expire_date で補正する（DB側が未設定/ずれている場合のみ更新される）
		if e.ExpireDate != nil {
			if err := UpdateOrderExpireDate(TableBuyOrders, e.OrderID, *e.ExpireDate); err != nil {
				log.Printf("[ERROR] SyncBuyOrders failed to update expire_date. order_id:%v err:%v", e.OrderID, err)
				failures = append(failures, SyncBuyOrderFailure{
					Operation: "expire_date", OrderID: e.OrderID, ProductCode: e.ProductCode, Side: e.Side,
					Price: e.Price, Size: e.Size, Strategy: e.Strategy, Err: err})
			}
		}
	}
	return failures
}

// 過去3日分の利益を取得する関数
func GetResults() (string, error) {
	if database.CurrentDriver() == "postgres" {
		return getResultsPostgres()
	}
	return getResultsMySQL()
}

func getResultsMySQL() (string, error) {
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
			a.parentid = b.order_id and a.status = 'FILLED'
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
		where a.parentid = b.order_id and a.status = 'FILLED'
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
	return formatResults(rows)
}

func getResultsPostgres() (string, error) {
	rows, err := AppDB.Query(`
		select
		 'Total' as date,
		 COALESCE(round((sum(average.profit) * 0.9989)::numeric, 2)::float, 0) as profit,
		 count(average.profit) as count,
		 COALESCE(round((avg(average.profit) * 0.9989)::numeric, 2)::float, 0) as ppt
		from
		(select
			TO_CHAR(a.updatetime, 'YYYY-MM-DD') as date,
			sum((a.price * a.size) - (b.price * b.size)) as profit
		from
			sell_orders a,
			buy_orders b
		where
			a.parentid = b.order_id and a.status = 'FILLED'
			and TO_CHAR(a.updatetime, 'YYYY-MM-DD') <> '2020-01-01'
		group by TO_CHAR(a.updatetime, 'YYYY-MM-DD')) average

		union all

		select
		 result.date as date,
		 COALESCE(round((sum(result.profit) * 0.9989)::numeric, 2)::float, 0) as profit,
		 count(result.profit) as count,
		 COALESCE(round((sum(result.profit) / count(result.profit) * 0.9989)::numeric, 2)::float, 0) as ppt
		from
		(select
			TO_CHAR(a.updatetime, 'YYYY-MM-DD') as date,
			(a.price * a.size) - (b.price * b.size) as profit
		from
			sell_orders a,
			buy_orders b
		where a.parentid = b.order_id and a.status = 'FILLED') result
		group by result.date
		order by date desc
		limit 4
		`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	return formatResults(rows)
}

type rowScanner interface {
	Next() bool
	Scan(...any) error
	Err() error
}

func formatResults(rows rowScanner) (string, error) {
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
	if err := rows.Err(); err != nil {
		return "", err
	}
	return sb.String(), nil
}
