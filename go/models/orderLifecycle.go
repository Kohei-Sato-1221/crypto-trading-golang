package models

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/database"
)

/*
OrderTable は注文テーブル名の限定型。

buy_orders / sell_orders で同じSQLを共有するためにテーブル名を文字列連結するが、
呼び出し側から任意の文字列を渡せないようにして SQL インジェクションの余地をなくす。
*/
type OrderTable string

const (
	TableBuyOrders  OrderTable = "buy_orders"
	TableSellOrders OrderTable = "sell_orders"
)

/*
ExchangeBitflyer は exchange カラムに入る Bitflyer の識別子。

buy_orders / sell_orders は OKEX サービスと共用のテーブルである
（cmds のエントリポイントで okex.TableName = "buy_orders" が設定される）。
Bitflyer 用のジョブが取引所を絞らずに UNFILLED を拾うと、OKEX 由来のレコードに対して
Bitflyer の CancelOrder を OKEX の product_code で叩くことになり、
毎日のSlackエラーが恒常化する。Bitflyer 専用のSELECTは必ずこの値で絞ること。

2026-09 時点の本番データは buy_orders / sell_orders とも全件 exchange='bitflyer' で
NULL は存在しないため、この条件で既存レコードが取りこぼされることはない。
*/
const ExchangeBitflyer = "bitflyer"

// isValid は定義済みのテーブル名かどうかを返す。
func (t OrderTable) isValid() bool {
	return t == TableBuyOrders || t == TableSellOrders
}

/*
UpdateOrderExpireDate は order_id を指定して expire_date を上書きする。

取引所APIが返す expire_date による補正（syncBuyOrders）に使う。
値は必ずUTCへ正規化して保存する。既に同じ値が入っている場合は更新しない
（updatetime が無用に更新されるのを避けるため）。
*/
func UpdateOrderExpireDate(table OrderTable, orderID string, expireDate time.Time) error {
	if !table.isValid() {
		return fmt.Errorf("UpdateOrderExpireDate: invalid table: %s", table)
	}
	if orderID == "" {
		return errors.New("UpdateOrderExpireDate: order_id is empty")
	}

	utcExpireDate := expireDate.UTC()

	var query string
	var args []any
	if database.CurrentDriver() == "postgres" {
		query = fmt.Sprintf(
			`UPDATE %s SET expire_date = $1 WHERE order_id = $2 AND (expire_date IS NULL OR expire_date <> $1)`,
			table)
		args = []any{utcExpireDate, orderID}
	} else {
		query = fmt.Sprintf(
			`UPDATE %s SET expire_date = ? WHERE order_id = ? AND (expire_date IS NULL OR expire_date <> ?)`,
			table)
		args = []any{utcExpireDate, orderID, utcExpireDate}
	}

	result, err := AppDB.Exec(query, args...)
	if err != nil {
		log.Printf("[ERROR] UpdateOrderExpireDate table:%s order_id:%s expire_date:%s err:%v",
			table, orderID, utcExpireDate.Format(time.RFC3339), err)
		return err
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr == nil && rows > 0 {
		log.Printf("UpdateOrderExpireDate table:%s order_id:%s expire_date(UTC):%s updated:%d",
			table, orderID, utcExpireDate.Format(time.RFC3339), rows)
	}
	return nil
}

/*
remarks に埋め込む機械可読トークン。

日本語散文でのマッチは表記ゆれ・文言変更で壊れるため、判定には必ずこのトークンを使う。
*/
const (
	/*
		RemarkManualHold は手動保有へ移管済みのレコードのマーカー。

		2026-09-06 の復旧作業で手動保有へ移した130レコード(buy_orders 65件 / sell_orders 65件)に
		付与されている。ボットは一切関与せずユーザーが手動で売却するため、
		reconcileJob の残高突合では内訳として別枠表示するだけで乖離アラートの対象にしない。
		判定は必ずこのトークンで行い、日本語散文(「手動保有へ移管」等)でのマッチはしない
		（表記ゆれ・文言変更で壊れるため）。
	*/
	RemarkManualHold = "[MANUAL_HOLD]"

	/*
		RemarkRolloverPending は「キャンセルは成功したが再発注に失敗した」売り注文のマーカー。

		ローリング(rolloverSellOrderJob)がまだ再試行しうる間だけ expireSweepJob の対象から除外する。
		再試行と失効sweepが競合し、再発注待ちのレコードを勝手にCANCELLEDへ落とすのを防ぐため。
		ローリングの窓を過ぎたレコードは除外を解いて sweep の担当に移す
		（除外したままにするとどのジョブにも拾われず恒久的に滞留するため。
		詳細は GetExpiredUnfilledOrders / GetUnfilledOrdersWithoutExpireDate のコメントを参照）。
	*/
	RemarkRolloverPending = "[ROLLOVER_PENDING]"
)

// rolloverPendingLikePattern は RemarkRolloverPending を含むremarksを除外するためのLIKEパターン。
// SQLのLIKEにおけるワイルドカードは % と _ のみで、[ ] は特殊文字ではないためエスケープは不要。
var rolloverPendingLikePattern = "%" + RemarkRolloverPending + "%"

// manualHoldLikePattern は RemarkManualHold を含むremarksを抽出・除外するためのLIKEパターン。
var manualHoldLikePattern = "%" + RemarkManualHold + "%"

/*
OrderRecord は buy_orders / sell_orders の共通カラムを表すレコード。

失効検出(expireSweepJob)・能動キャンセル(cancelBuyOrderJob)のように
両テーブルを同じロジックで扱う処理で利用する。
ParentID は sell_orders のみ値を持ち、buy_orders では空文字になる。
時刻はすべてUTCとして扱う。
*/
type OrderRecord struct {
	Table       OrderTable
	OrderID     string
	ParentID    string
	ProductCode string
	Side        string
	Price       float64
	Size        float64
	Exchange    string
	Status      string
	Remarks     string
	ExpireDate  *time.Time // UTC。DB上NULL、または値を解釈できなかった場合はnil
	Timestamp   time.Time  // UTC。TimestampValid が false のときはゼロ値

	/*
		TimestampValid は timestamp をUTCの time.Time へ変換できたかどうか。

		false になるのはDB上NULLの場合か、文字列を解釈できなかった場合。
		呼び出し側はゼロ値の Timestamp を「十分古い」と解釈してはならない
		（cancelBuyOrderJob の order.Timestamp.After(threshold) は、ゼロ値だと
		常に false になりキャンセル側へ倒れてしまう）。判定不能なレコードは
		処理をスキップして通知すること。
	*/
	TimestampValid bool
}

// orderRecordColumns は OrderRecord を組み立てるためのSELECT句を返す。
// buy_orders には parentid 列が無いため空文字を埋めて列数を揃える。
func orderRecordColumns(table OrderTable) string {
	if table == TableSellOrders {
		return "order_id, parentid, product_code, side, price, size, exchange, status, remarks, expire_date, timestamp"
	}
	return "order_id, '' AS parentid, product_code, side, price, size, exchange, status, remarks, expire_date, timestamp"
}

/*
toUTCTime はDBから取得した時刻値をUTCの time.Time に変換する。

PostgreSQL(pgx)は timestamp を time.Time で返すが、MySQLドライバは
DSNに parseTime=true が無い場合 []byte を返すため、両方を受け付ける。
変換できない場合（NULL、または解釈できない文字列）は ok=false を返す。
この ok は握り潰さず OrderRecord.TimestampValid として呼び出し側へ伝えること。
ゼロ値のまま通すと、時刻比較で「十分古い」と誤解釈されてキャンセル等の
破壊的な処理に倒れてしまう。
*/
func toUTCTime(value any) (time.Time, bool) {
	switch v := value.(type) {
	case nil:
		return time.Time{}, false
	case time.Time:
		return v.UTC(), true
	case []byte:
		return parseDBTimeString(string(v))
	case string:
		return parseDBTimeString(v)
	}
	return time.Time{}, false
}

// parseDBTimeString はDBが文字列で返した時刻をUTCとして解釈する。
func parseDBTimeString(value string) (time.Time, bool) {
	layouts := []string{
		"2006-01-02 15:04:05.999999999",
		"2006-01-02T15:04:05.999999999",
		time.RFC3339Nano,
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, value, time.UTC); err == nil {
			return t.UTC(), true
		}
	}
	log.Printf("[ERROR] failed to parse timestamp from DB: %s", value)
	return time.Time{}, false
}

// scanOrderRecords は orderRecordColumns() の並びで取得した行を OrderRecord へ変換する。
func scanOrderRecords(table OrderTable, rows *sql.Rows) ([]OrderRecord, error) {
	var records []OrderRecord
	for rows.Next() {
		var (
			orderID     sql.NullString
			parentID    sql.NullString
			productCode sql.NullString
			side        sql.NullString
			price       sql.NullFloat64
			size        sql.NullFloat64
			exchange    sql.NullString
			status      sql.NullString
			remarks     sql.NullString
			expireDate  any
			timestamp   any
		)
		if err := rows.Scan(&orderID, &parentID, &productCode, &side, &price, &size,
			&exchange, &status, &remarks, &expireDate, &timestamp); err != nil {
			return nil, err
		}
		record := OrderRecord{
			Table:       table,
			OrderID:     orderID.String,
			ParentID:    parentID.String,
			ProductCode: productCode.String,
			Side:        side.String,
			Price:       price.Float64,
			Size:        size.Float64,
			Exchange:    exchange.String,
			Status:      status.String,
			Remarks:     remarks.String,
		}
		if expire, ok := toUTCTime(expireDate); ok {
			record.ExpireDate = &expire
		}
		// 変換の成否は握り潰さず呼び出し側へ伝える（ゼロ値を有効な時刻として扱わせない）
		record.Timestamp, record.TimestampValid = toUTCTime(timestamp)
		records = append(records, record)
	}
	return records, rows.Err()
}

/*
GetExpiredUnfilledOrders は有効期限を過ぎた未約定レコードを返す（失効検出の方式A）。

条件:
  - status = 'UNFILLED' かつ order_id が空でない
    → 手動保有へ移管した130レコード(status='CANCELLED' / 'FILLED(SELL ORDER PLACED)')は構造的に対象外
  - expire_date IS NOT NULL かつ expire_date < (now - grace)
    → grace は時計ずれを吸収する猶予時間

RemarkRolloverPending が付いたレコードを除外してはならない。
ローリングの抽出条件(GetSellOrdersToRollover)は expire_date > now という下限を持つため、
期限を過ぎたレコードはそもそもローリングの対象にならない。ここで併せて除外すると
「キャンセル成功・再発注失敗のまま期限を過ぎたレコード」がどのジョブにも拾われず、
status='UNFILLED' のまま恒久的に滞留してスロットを食い潰す（2026-05の障害と同じ構造）。
本クエリの窓(expire_date < now - grace)とローリングの窓(expire_date > now)は
排他なので、除外しなくても再発注待ちレコードと競合しない。
（expire_date が NULL の旧レコードについては窓が重なりうるため、
GetUnfilledOrdersWithoutExpireDate 側で rolloverRetryAfter による切り分けを行う）

now はUTCへ正規化して比較する。limit は一度に処理する件数の安全弁。
*/
func GetExpiredUnfilledOrders(table OrderTable, now time.Time, grace time.Duration, limit int) ([]OrderRecord, error) {
	if !table.isValid() {
		return nil, fmt.Errorf("GetExpiredUnfilledOrders: invalid table: %s", table)
	}
	if limit <= 0 {
		return nil, fmt.Errorf("GetExpiredUnfilledOrders: invalid limit: %d", limit)
	}
	threshold := now.UTC().Add(-grace)

	var query string
	if database.CurrentDriver() == "postgres" {
		query = fmt.Sprintf(`SELECT %s FROM %s
			WHERE status = 'UNFILLED' AND order_id <> ''
			  AND expire_date IS NOT NULL AND expire_date < $1
			ORDER BY expire_date LIMIT $2`, orderRecordColumns(table), table)
	} else {
		query = fmt.Sprintf(`SELECT %s FROM %s
			WHERE status = 'UNFILLED' AND order_id <> ''
			  AND expire_date IS NOT NULL AND expire_date < ?
			ORDER BY expire_date LIMIT ?`, orderRecordColumns(table), table)
	}

	rows, err := AppDB.Query(query, threshold, limit)
	if err != nil {
		log.Printf("[ERROR] GetExpiredUnfilledOrders table:%s threshold(UTC):%s err:%v",
			table, threshold.Format(time.RFC3339), err)
		return nil, err
	}
	defer rows.Close()
	return scanOrderRecords(table, rows)
}

/*
GetUnfilledOrdersWithoutExpireDate は expire_date が未設定の未約定レコードを返す（失効検出の方式B）。

expire_date カラム追加より前に取り込まれた旧レコードの救済に使う。
呼び出し側は取引所のACTIVE/COMPLETED一覧と突合し、どちらにも存在しないものだけを失効とみなすこと。

rolloverRetryAfter は「ローリングがまだ再試行しうる」timestamp の下限(UTC)。
expire_date が NULL のレコードに対するローリングの抽出条件は
timestamp > now - (fallbackDays + daysBefore) 日 なので、同じ値を渡すこと。
RemarkRolloverPending が付いたレコードは、この境界より新しい間だけ除外する
（再発注待ちのレコードを sweep が勝手にCANCELLEDへ落とすのを防ぐため）。
境界より古くなればローリングは二度と対象にしないため、除外を解いて sweep の担当に移す。
除外したままにすると、キャンセル成功・再発注失敗のレコードがどのジョブにも拾われず
status='UNFILLED' のまま恒久的に滞留する。ゼロ値を渡した場合は全期間で除外する（従来動作）。
*/
func GetUnfilledOrdersWithoutExpireDate(table OrderTable, productCode string,
	rolloverRetryAfter time.Time, limit int) ([]OrderRecord, error) {

	if !table.isValid() {
		return nil, fmt.Errorf("GetUnfilledOrdersWithoutExpireDate: invalid table: %s", table)
	}
	if productCode == "" {
		return nil, errors.New("GetUnfilledOrdersWithoutExpireDate: product_code is empty")
	}
	if limit <= 0 {
		return nil, fmt.Errorf("GetUnfilledOrdersWithoutExpireDate: invalid limit: %d", limit)
	}
	retryAfter := rolloverRetryAfter.UTC()

	var query string
	if database.CurrentDriver() == "postgres" {
		query = fmt.Sprintf(`SELECT %s FROM %s
			WHERE status = 'UNFILLED' AND order_id <> ''
			  AND expire_date IS NULL AND product_code = $1
			  AND (remarks IS NULL OR remarks NOT LIKE $2 OR timestamp <= $3)
			ORDER BY timestamp LIMIT $4`, orderRecordColumns(table), table)
	} else {
		query = fmt.Sprintf(`SELECT %s FROM %s
			WHERE status = 'UNFILLED' AND order_id <> ''
			  AND expire_date IS NULL AND product_code = ?
			  AND (remarks IS NULL OR remarks NOT LIKE ? OR timestamp <= ?)
			ORDER BY timestamp LIMIT ?`, orderRecordColumns(table), table)
	}

	rows, err := AppDB.Query(query, productCode, rolloverPendingLikePattern, retryAfter, limit)
	if err != nil {
		log.Printf("[ERROR] GetUnfilledOrdersWithoutExpireDate table:%s product_code:%s rolloverRetryAfter(UTC):%s err:%v",
			table, productCode, retryAfter.Format(time.RFC3339), err)
		return nil, err
	}
	defer rows.Close()
	return scanOrderRecords(table, rows)
}

/*
GetUnfilledBuyOrderRecords は未約定の Bitflyer の買い注文を expire_date 付きで返す。

cancelBuyOrderJob が「有効期限」と「経過日数」の両方で能動キャンセルの要否を判定するために使う。
GORMの GetUnfilledBuyOrders() は expire_date を保持しないため、こちらを使うこと。

buy_orders は OKEX サービスと共用のテーブルなので exchange = ExchangeBitflyer で必ず絞る。
絞らないと OKEX 由来の UNFILLED レコードに対して Bitflyer の CancelOrder を
OKEX の product_code で叩き、毎日のSlackエラーが恒常化する。
*/
func GetUnfilledBuyOrderRecords(limit int) ([]OrderRecord, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("GetUnfilledBuyOrderRecords: invalid limit: %d", limit)
	}

	var query string
	if database.CurrentDriver() == "postgres" {
		query = fmt.Sprintf(`SELECT %s FROM %s
			WHERE status = 'UNFILLED' AND order_id <> '' AND exchange = $1
			ORDER BY timestamp LIMIT $2`, orderRecordColumns(TableBuyOrders), TableBuyOrders)
	} else {
		query = fmt.Sprintf(`SELECT %s FROM %s
			WHERE status = 'UNFILLED' AND order_id <> '' AND exchange = ?
			ORDER BY timestamp LIMIT ?`, orderRecordColumns(TableBuyOrders), TableBuyOrders)
	}

	rows, err := AppDB.Query(query, ExchangeBitflyer, limit)
	if err != nil {
		log.Printf("[ERROR] GetUnfilledBuyOrderRecords err:%v", err)
		return nil, err
	}
	defer rows.Close()
	return scanOrderRecords(TableBuyOrders, rows)
}

/*
MarkOrderCancelledWithRemark は status を CANCELLED にし、remarks に追記する。

status = 'UNFILLED' の行のみを対象にすることで、
他ジョブが同時に FILLED / CANCELLED へ更新したレコードを上書きしない。
更新行数が0の場合はエラーにせず、その旨をログに残す。
*/
func MarkOrderCancelledWithRemark(table OrderTable, orderID, remark string) error {
	if !table.isValid() {
		return fmt.Errorf("MarkOrderCancelledWithRemark: invalid table: %s", table)
	}
	if orderID == "" {
		return errors.New("MarkOrderCancelledWithRemark: order_id is empty")
	}

	var query string
	var args []any
	if database.CurrentDriver() == "postgres" {
		query = fmt.Sprintf(
			`UPDATE %s SET status = '%s', remarks = COALESCE(remarks, '') || $1 WHERE order_id = $2 AND status = '%s'`,
			table, OrderStatusCancelled, OrderStatusUnfilled)
	} else {
		query = fmt.Sprintf(
			`UPDATE %s SET status = '%s', remarks = CONCAT(COALESCE(remarks, ''), ?) WHERE order_id = ? AND status = '%s'`,
			table, OrderStatusCancelled, OrderStatusUnfilled)
	}
	args = []any{remark, orderID}

	result, err := AppDB.Exec(query, args...)
	if err != nil {
		log.Printf("[ERROR] MarkOrderCancelledWithRemark table:%s order_id:%s err:%v", table, orderID, err)
		return err
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		log.Printf("MarkOrderCancelledWithRemark table:%s order_id:%s updated(rows unknown)", table, orderID)
		return nil
	}
	if rows == 0 {
		log.Printf("MarkOrderCancelledWithRemark table:%s order_id:%s no row updated (already updated by another job?)", table, orderID)
		return nil
	}
	log.Printf("MarkOrderCancelledWithRemark table:%s order_id:%s updated:%d", table, orderID, rows)
	return nil
}

/*
AppendOrderRemark は remarks への追記のみを行う（status は変更しない）。

ローリングの再試行マーカー(RemarkRolloverPending)の付与に使う。
同じ文言が既に含まれている場合は追記しない（日次ジョブで何度も呼ばれても
remarks が際限なく伸びないようにするため）。この判定はUPDATEのWHERE句で行うので、
SELECT→UPDATEの間に他ジョブが追記しても二重追記にはならない。
*/
func AppendOrderRemark(table OrderTable, orderID, remark string) error {
	if !table.isValid() {
		return fmt.Errorf("AppendOrderRemark: invalid table: %s", table)
	}
	if orderID == "" {
		return errors.New("AppendOrderRemark: order_id is empty")
	}
	if remark == "" {
		return errors.New("AppendOrderRemark: remark is empty")
	}
	// LIKEのワイルドカードは % と _ のみ。マーカーに含まれる [ ] はエスケープ不要
	likePattern := "%" + remark + "%"

	var query string
	if database.CurrentDriver() == "postgres" {
		query = fmt.Sprintf(
			`UPDATE %s SET remarks = COALESCE(remarks, '') || $1 WHERE order_id = $2 AND (remarks IS NULL OR remarks NOT LIKE $3)`,
			table)
	} else {
		query = fmt.Sprintf(
			`UPDATE %s SET remarks = CONCAT(COALESCE(remarks, ''), ?) WHERE order_id = ? AND (remarks IS NULL OR remarks NOT LIKE ?)`,
			table)
	}

	result, err := AppDB.Exec(query, remark, orderID, likePattern)
	if err != nil {
		log.Printf("[ERROR] AppendOrderRemark table:%s order_id:%s remark:%s err:%v", table, orderID, remark, err)
		return err
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr == nil {
		log.Printf("AppendOrderRemark table:%s order_id:%s remark:%s updated:%d", table, orderID, remark, rows)
	}
	return nil
}

/*
SellOrderRecord はローリング対象の売り注文レコード。

sell_orders は OrderRecord と同じカラム構成（parentid を含む）で扱えるため型エイリアスとする。
別構造体にするとスキャン処理を二重管理することになり、カラム追加時にずれるため。
*/
type SellOrderRecord = OrderRecord

/*
GetSellOrdersToRollover はローリング（キャンセル→同条件で再発注）の対象となる売り注文を返す。

Bitflyerの注文有効期限は最長30日（43200分）で、無期限注文は作れない。
そのため期限の daysBefore 日前になった売り注文を巻き直して実質無期限化する。

条件:
  - exchange = ExchangeBitflyer
    → sell_orders は OKEX サービスと共用のテーブルであり、絞らないと存在しない通貨ペアで
    Bitflyer の一覧取得・キャンセルを試みて毎日のSlackエラーが恒常化する
  - status = 'UNFILLED' かつ order_id が空でない
    → 手動保有へ移管した65件(sell_orders.status='CANCELLED')は構造的に対象外になる。
    同様に約定済み(FILLED)・キャンセル済みのレコードも対象にならない
  - expire_date IS NOT NULL の場合: now < expire_date < now + daysBefore 日
    → 上限だけでなく下限も設ける。既に期限を過ぎたレコードは取引所側に注文が存在せず
    キャンセルAPIが必ず失敗するため、ローリングではなく expireSweepJob の担当とする
  - expire_date IS NULL の旧レコードの場合:
    now - (fallbackDays + daysBefore) 日 < timestamp < now - fallbackDays 日
    → expire_date カラム追加前に発注されたレコードの救済。売り注文の期限は発注時点から30日なので
    fallbackDays(既定27日)を経過していれば期限が近いとみなせる。
    下限の (fallbackDays + daysBefore) 日 = 想定寿命30日であり、これを超えて古いレコードは
    既に失効しているとみなして対象外にする（expire_date ありの場合の下限と同じ考え方）

RemarkRolloverPending が付いたレコードは除外しない。
「キャンセルは成功したが再発注に失敗した」レコードを次回ジョブで再試行するためである
（除外しているのは expireSweepJob 側）。

now はUTCへ正規化して比較する。limit は1回の実行で処理する件数の安全弁
（レート制限と、一度に巻き直す注文数の上限を兼ねる）。timestamp の昇順で古いものから返す。
*/
func GetSellOrdersToRollover(now time.Time, daysBefore, fallbackDays, limit int) ([]SellOrderRecord, error) {
	if daysBefore < 0 {
		return nil, fmt.Errorf("GetSellOrdersToRollover: invalid daysBefore: %d", daysBefore)
	}
	if fallbackDays <= 0 {
		return nil, fmt.Errorf("GetSellOrdersToRollover: invalid fallbackDays: %d", fallbackDays)
	}
	if limit <= 0 {
		return nil, fmt.Errorf("GetSellOrdersToRollover: invalid limit: %d", limit)
	}

	utcNow := now.UTC()
	// 期限が「今より後」かつ「daysBefore日以内」のものだけを対象にする。
	// 既に失効したレコードは取引所に注文が存在せずキャンセルできないため expireSweepJob に委ねる
	expireLower := utcNow
	expireUpper := utcNow.AddDate(0, 0, daysBefore)
	// expire_date未設定の旧レコードは timestamp で代用する。
	// 上限: fallbackDays日を経過（＝期限が近い）／下限: 想定寿命(fallbackDays+daysBefore=30日)以内
	fallbackUpper := utcNow.AddDate(0, 0, -fallbackDays)
	fallbackLower := utcNow.AddDate(0, 0, -(fallbackDays + daysBefore))

	var query string
	if database.CurrentDriver() == "postgres" {
		query = fmt.Sprintf(`SELECT %s FROM %s
			WHERE status = 'UNFILLED' AND order_id <> '' AND exchange = $1
			  AND ((expire_date IS NOT NULL AND expire_date > $2 AND expire_date < $3)
			    OR (expire_date IS NULL AND timestamp < $4 AND timestamp > $5))
			ORDER BY timestamp LIMIT $6`, orderRecordColumns(TableSellOrders), TableSellOrders)
	} else {
		query = fmt.Sprintf(`SELECT %s FROM %s
			WHERE status = 'UNFILLED' AND order_id <> '' AND exchange = ?
			  AND ((expire_date IS NOT NULL AND expire_date > ? AND expire_date < ?)
			    OR (expire_date IS NULL AND timestamp < ? AND timestamp > ?))
			ORDER BY timestamp LIMIT ?`, orderRecordColumns(TableSellOrders), TableSellOrders)
	}

	rows, err := AppDB.Query(query, ExchangeBitflyer, expireLower, expireUpper, fallbackUpper, fallbackLower, limit)
	if err != nil {
		log.Printf("[ERROR] GetSellOrdersToRollover expire_date(UTC):%s〜%s timestamp(UTC):%s〜%s err:%v",
			expireLower.Format(time.RFC3339), expireUpper.Format(time.RFC3339),
			fallbackLower.Format(time.RFC3339), fallbackUpper.Format(time.RFC3339), err)
		return nil, err
	}
	defer rows.Close()
	return scanOrderRecords(TableSellOrders, rows)
}

/*
RolloverSellOrder は「旧レコードのCANCELLED化」と「新レコードのINSERT」を単一トランザクションで実行する。

呼び出しの前提: 取引所側で旧注文のキャンセルが成立し、新しい売り注文の発注が成功していること。

  - 旧レコード: status を CANCELLED にし、remarks に再発注先(newOrderID)を追記する
  - 新レコード: parentid（元の買い注文ID）・product_code・price・size・exchange を旧レコードから引き継ぐ。
    parentid を引き継がないと損益計算(sell_orders.parentid = buy_orders.order_id の結合)が壊れる
  - 新レコードの remarks には「{旧order_id}の売り注文の再注文」を記録する
  - 新レコードの expire_date にはUTCの期限を保存する（次回のローリング判定に使う）

旧レコードのUPDATEは status='UNFILLED' の行だけを対象にする。更新行数が0の場合は
他ジョブがFILLED/CANCELLEDへ変更した後ということなので、二重売りの疑いがあるため
ロールバックしてエラーを返す（新レコードのINSERTも行わない）。
返却するエラーには newOrderID を含めるので、呼び出し側はSlack通知で手動対応を促せる。
*/
func RolloverSellOrder(old SellOrderRecord, newOrderID string, newPrice float64, newExpire time.Time) error {
	if old.OrderID == "" {
		return errors.New("RolloverSellOrder: old order_id is empty")
	}
	if newOrderID == "" {
		return errors.New("RolloverSellOrder: new order_id is empty")
	}

	utcNow := time.Now().UTC()
	utcExpire := newExpire.UTC()
	oldRemark := fmt.Sprintf(" / rolled over to %s at %s", newOrderID, utcNow.Format(time.RFC3339))
	newRemark := fmt.Sprintf("%sの売り注文の再注文", old.OrderID)

	var updateQuery, insertQuery string
	if database.CurrentDriver() == "postgres" {
		updateQuery = fmt.Sprintf(
			`UPDATE %s SET status = '%s', remarks = COALESCE(remarks, '') || $1 WHERE order_id = $2 AND status = '%s'`,
			TableSellOrders, OrderStatusCancelled, OrderStatusUnfilled)
		insertQuery = fmt.Sprintf(
			`INSERT INTO %s (parentid, order_id, product_code, side, price, size, exchange, remarks, expire_date)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, TableSellOrders)
	} else {
		updateQuery = fmt.Sprintf(
			`UPDATE %s SET status = '%s', remarks = CONCAT(COALESCE(remarks, ''), ?) WHERE order_id = ? AND status = '%s'`,
			TableSellOrders, OrderStatusCancelled, OrderStatusUnfilled)
		insertQuery = fmt.Sprintf(
			`INSERT INTO %s (parentid, order_id, product_code, side, price, size, exchange, remarks, expire_date)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, TableSellOrders)
	}

	tx, err := AppDB.Begin()
	if err != nil {
		log.Printf("[ERROR] RolloverSellOrder failed to begin tx. old_order_id:%s new_order_id:%s err:%v",
			old.OrderID, newOrderID, err)
		return err
	}

	result, err := tx.Exec(updateQuery, oldRemark, old.OrderID)
	if err != nil {
		_ = tx.Rollback()
		log.Printf("[ERROR] RolloverSellOrder failed to update old record. old_order_id:%s new_order_id:%s err:%v",
			old.OrderID, newOrderID, err)
		return fmt.Errorf("failed to mark old sell order as CANCELLED (old_order_id:%s new_order_id:%s): %w",
			old.OrderID, newOrderID, err)
	}
	// 更新行数を確認できない場合も「UNFILLEDの行を更新できた」と断定できないため、
	// 二重売りの疑いを残さないようロールバックして失敗側に倒す
	rows, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		_ = tx.Rollback()
		return fmt.Errorf("could not confirm the update of the old sell order. rolled back without inserting the new record "+
			"(old_order_id:%s new_order_id:%s): %w", old.OrderID, newOrderID, rowsErr)
	}
	if rows == 0 {
		_ = tx.Rollback()
		return fmt.Errorf("old sell order is no longer UNFILLED. rolled back without inserting the new record "+
			"(old_order_id:%s new_order_id:%s). 取引所には新しい売り注文が残っている可能性があるため手動確認が必要です",
			old.OrderID, newOrderID)
	}

	side := old.Side
	if side == "" {
		side = "SELL"
	}
	if _, err := tx.Exec(insertQuery, old.ParentID, newOrderID, old.ProductCode, side,
		newPrice, old.Size, old.Exchange, newRemark, utcExpire); err != nil {
		_ = tx.Rollback()
		log.Printf("[ERROR] RolloverSellOrder failed to insert new record. old_order_id:%s new_order_id:%s err:%v",
			old.OrderID, newOrderID, err)
		return fmt.Errorf("failed to insert rolled over sell order (old_order_id:%s new_order_id:%s parentid:%s): %w",
			old.OrderID, newOrderID, old.ParentID, err)
	}

	if err := tx.Commit(); err != nil {
		log.Printf("[ERROR] RolloverSellOrder failed to commit. old_order_id:%s new_order_id:%s err:%v",
			old.OrderID, newOrderID, err)
		return fmt.Errorf("failed to commit rollover (old_order_id:%s new_order_id:%s): %w",
			old.OrderID, newOrderID, err)
	}

	log.Printf("RolloverSellOrder done. old_order_id:%s -> new_order_id:%s parentid:%s %s price:%.2f size:%v expire_date(UTC):%s",
		old.OrderID, newOrderID, old.ParentID, old.ProductCode, newPrice, old.Size, utcExpire.Format(time.RFC3339))
	return nil
}

/*
UpdateOrderSizeWithRemark は未約定レコードの size を実際に売れる数量へ補正し、remarks に経緯を追記する。

売り注文が部分約定していた場合、DB上の size（発注時の数量）と実際にまだ売れる数量
（outstanding_size）がずれる。ローリングは「残っている数量だけを売りに出す」方針のため、
キャンセルを実行する前にDBの size を実数量へ合わせておく。

事前に補正しておくことが重要である。キャンセル済みの注文はBitflyerのAPIから即座に消えるため
（実測確認済み: キャンセル直後の個別照会は空配列を返す）、再発注に失敗して
[ROLLOVER_PENDING] のまま翌日へ持ち越された場合には outstanding_size を再取得できない。
DB側に実数量を残しておくことで、元の過大な数量で再発注してしまう事故を防げる。

status = 'UNFILLED' の行のみを対象にし、更新行数が0または確認できない場合はエラーを返す
（数量を補正できていないまま発注処理へ進ませないため）。
*/
func UpdateOrderSizeWithRemark(table OrderTable, orderID string, size float64, remark string) error {
	if !table.isValid() {
		return fmt.Errorf("UpdateOrderSizeWithRemark: invalid table: %s", table)
	}
	if orderID == "" {
		return errors.New("UpdateOrderSizeWithRemark: order_id is empty")
	}
	if size <= 0 {
		return fmt.Errorf("UpdateOrderSizeWithRemark: invalid size: %v", size)
	}

	var query string
	if database.CurrentDriver() == "postgres" {
		query = fmt.Sprintf(
			`UPDATE %s SET size = $1, remarks = COALESCE(remarks, '') || $2 WHERE order_id = $3 AND status = '%s'`,
			table, OrderStatusUnfilled)
	} else {
		query = fmt.Sprintf(
			`UPDATE %s SET size = ?, remarks = CONCAT(COALESCE(remarks, ''), ?) WHERE order_id = ? AND status = '%s'`,
			table, OrderStatusUnfilled)
	}

	result, err := AppDB.Exec(query, size, remark, orderID)
	if err != nil {
		log.Printf("[ERROR] UpdateOrderSizeWithRemark table:%s order_id:%s size:%v err:%v", table, orderID, size, err)
		return err
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("could not confirm the size update (table:%s order_id:%s size:%v): %w", table, orderID, size, rowsErr)
	}
	if rows == 0 {
		return fmt.Errorf("no UNFILLED row to update the size (table:%s order_id:%s size:%v)", table, orderID, size)
	}
	log.Printf("UpdateOrderSizeWithRemark table:%s order_id:%s size:%v updated:%d", table, orderID, size, rows)
	return nil
}

/*
GetUnfilledOrderIDs は指定通貨ペアの未約定レコードの order_id 一覧を返す。

reconcileJob が取引所のACTIVE注文一覧とDBを突合するために使う。
status = 'UNFILLED' かつ order_id が空でない行のみを対象にするため、
手動保有へ移管した130レコード(CANCELLED / FILLED(SELL ORDER PLACED))は含まれない。
limit は全件取得を避けるための安全弁。上限に達した場合は2番目の戻り値 truncated を
true にして返す。打ち切りが起きると突合結果が実態とずれるため、呼び出し側は
これをSlackへ通知すること（ローカルログだけでは無言の停止に気づけない）。
*/
func GetUnfilledOrderIDs(table OrderTable, productCode string, limit int) ([]string, bool, error) {
	if !table.isValid() {
		return nil, false, fmt.Errorf("GetUnfilledOrderIDs: invalid table: %s", table)
	}
	if productCode == "" {
		return nil, false, errors.New("GetUnfilledOrderIDs: product_code is empty")
	}
	if limit <= 0 {
		return nil, false, fmt.Errorf("GetUnfilledOrderIDs: invalid limit: %d", limit)
	}

	var query string
	if database.CurrentDriver() == "postgres" {
		query = fmt.Sprintf(`SELECT order_id FROM %s
			WHERE status = 'UNFILLED' AND order_id <> '' AND product_code = $1
			ORDER BY timestamp LIMIT $2`, table)
	} else {
		query = fmt.Sprintf(`SELECT order_id FROM %s
			WHERE status = 'UNFILLED' AND order_id <> '' AND product_code = ?
			ORDER BY timestamp LIMIT ?`, table)
	}

	rows, err := AppDB.Query(query, productCode, limit)
	if err != nil {
		log.Printf("[ERROR] GetUnfilledOrderIDs table:%s product_code:%s err:%v", table, productCode, err)
		return nil, false, err
	}
	defer rows.Close()

	orderIDs := make([]string, 0)
	scanned := 0
	for rows.Next() {
		var orderID sql.NullString
		if err := rows.Scan(&orderID); err != nil {
			return nil, false, err
		}
		scanned++
		if orderID.String != "" {
			orderIDs = append(orderIDs, orderID.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	// 空文字の order_id は除外しているため、打ち切りの判定には取得行数(scanned)を使う
	truncated := scanned >= limit
	if truncated {
		log.Printf("[ERROR] GetUnfilledOrderIDs reached the limit. table:%s product_code:%s limit:%d", table, productCode, limit)
	}
	return orderIDs, truncated, nil
}

/*
OrderIDExists は指定した order_id のレコードがテーブルに存在するかを返す。

取引所には存在するがDBに紐づかない注文（オーファン注文）の判定に使う。
status は問わない。ローリングの再発注でレスポンスを取りこぼした場合、
取引所側にだけ注文が残るため、その注文を「未知の注文」と判定するために参照する。
*/
func OrderIDExists(table OrderTable, orderID string) (bool, error) {
	if !table.isValid() {
		return false, fmt.Errorf("OrderIDExists: invalid table: %s", table)
	}
	if orderID == "" {
		return false, errors.New("OrderIDExists: order_id is empty")
	}

	var query string
	if database.CurrentDriver() == "postgres" {
		query = fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s WHERE order_id = $1)`, table)
	} else {
		query = fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s WHERE order_id = ?)`, table)
	}

	var exists bool
	if err := AppDB.QueryRow(query, orderID).Scan(&exists); err != nil {
		log.Printf("[ERROR] OrderIDExists table:%s order_id:%s err:%v", table, orderID, err)
		return false, err
	}
	return exists, nil
}

/*
GetUnfilledOrdersWithRemark は remarks に指定文言を含む未約定レコードを返す。

reconcileJob が RemarkRolloverPending（キャンセル成功・再発注失敗）のマーカーが
消えずに残っているレコード、すなわちローリングが繰り返し失敗して
「現物を保有したまま売り注文が無い」状態に近づいているレコードを検知するために使う。
*/
func GetUnfilledOrdersWithRemark(table OrderTable, remark string, limit int) ([]OrderRecord, error) {
	if !table.isValid() {
		return nil, fmt.Errorf("GetUnfilledOrdersWithRemark: invalid table: %s", table)
	}
	if remark == "" {
		return nil, errors.New("GetUnfilledOrdersWithRemark: remark is empty")
	}
	if limit <= 0 {
		return nil, fmt.Errorf("GetUnfilledOrdersWithRemark: invalid limit: %d", limit)
	}
	// LIKEのワイルドカードは % と _ のみ。マーカーに含まれる [ ] はエスケープ不要
	likePattern := "%" + remark + "%"

	var query string
	if database.CurrentDriver() == "postgres" {
		query = fmt.Sprintf(`SELECT %s FROM %s
			WHERE status = 'UNFILLED' AND order_id <> '' AND remarks LIKE $1
			ORDER BY timestamp LIMIT $2`, orderRecordColumns(table), table)
	} else {
		query = fmt.Sprintf(`SELECT %s FROM %s
			WHERE status = 'UNFILLED' AND order_id <> '' AND remarks LIKE ?
			ORDER BY timestamp LIMIT ?`, orderRecordColumns(table), table)
	}

	rows, err := AppDB.Query(query, likePattern, limit)
	if err != nil {
		log.Printf("[ERROR] GetUnfilledOrdersWithRemark table:%s remark:%s err:%v", table, remark, err)
		return nil, err
	}
	defer rows.Close()
	return scanOrderRecords(table, rows)
}

/*
RecentBuyOrder は直近の買い注文の戦略値と発注時刻。

reconcileJob が「N日間ボットの発注が0件」を検知するために使う。
Strategy は enums.IsBotStrategy() でボット発注か手動取り込みかを判別する。
*/
type RecentBuyOrder struct {
	OrderID   string
	Strategy  int
	Timestamp time.Time // UTC
}

/*
GetRecentBuyOrders は買い注文を新しい順に limit 件返す（全件取得を避けるため必ず上限を設ける）。

戦略値の判定はSQLに埋め込まず呼び出し側(enums.IsBotStrategy)で行う。
戦略値の定義は enums に集約されており、SQLへ値のリストを二重に持たせないための措置。
*/
func GetRecentBuyOrders(limit int) ([]RecentBuyOrder, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("GetRecentBuyOrders: invalid limit: %d", limit)
	}

	var query string
	if database.CurrentDriver() == "postgres" {
		query = `SELECT order_id, strategy, timestamp FROM buy_orders
			WHERE order_id <> '' ORDER BY timestamp DESC LIMIT $1`
	} else {
		query = `SELECT order_id, strategy, timestamp FROM buy_orders
			WHERE order_id <> '' ORDER BY timestamp DESC LIMIT ?`
	}

	rows, err := AppDB.Query(query, limit)
	if err != nil {
		log.Printf("[ERROR] GetRecentBuyOrders limit:%d err:%v", limit, err)
		return nil, err
	}
	defer rows.Close()

	records := make([]RecentBuyOrder, 0, limit)
	for rows.Next() {
		var (
			orderID   sql.NullString
			strategy  sql.NullInt64
			timestamp any
		)
		if err := rows.Scan(&orderID, &strategy, &timestamp); err != nil {
			return nil, err
		}
		record := RecentBuyOrder{
			OrderID:  orderID.String,
			Strategy: int(strategy.Int64),
		}
		if ts, ok := toUTCTime(timestamp); ok {
			record.Timestamp = ts
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

/*
ExpectedHolding は product_code ごとのDB上の想定保有量の内訳。

reconcileJob が取引所の実残高と突合するために使う。3つの内訳に分けるのは、
「乖離アラートの対象にすべきもの」と「そうでないもの」を分離するためである。

  - Bot:    ボットが追跡している保有量。売り注文が生きている分(sell_orders UNFILLED)と、
    約定済みでまだ売り注文を出していない分(buy_orders FILLED)の合計
  - Naked:  約定済みだが対応する売り注文が生存していない分（ローリング失敗や失効によるいわゆる裸の保有）。
    現物は手元にあるため保有量としては期待値に含めるが、要手動対応として別枠で可視化する
  - Manual: RemarkManualHold が付いた手動保有分。ボットは一切関与せずユーザーが任意のタイミングで
    手動売却するため、乖離アラートの判定からは完全に除外し、内訳表示にのみ使う
    （判定に含めると、売却された瞬間から「不足」側の乖離が恒久的に残りアラートが鳴り続ける）
*/
type ExpectedHolding struct {
	Bot    float64
	Naked  float64
	Manual float64
}

/*
Total は手動保有分を含む内訳の合計（DB上の全想定保有量）を返す。

⚠️ 残高の乖離判定にこの値を使ってはならない。Manual はユーザーが任意に売買するため、
判定に含めると手動売却後に恒久的な「不足」として検知され続ける。
乖離判定には必ず AlertTarget() を使うこと。Total() は表示・調査用の参考値である。
*/
func (h ExpectedHolding) Total() float64 {
	return h.Bot + h.Naked + h.Manual
}

/*
AlertTarget は残高の乖離判定に使う想定保有量（Bot + Naked）を返す。

ボットが売るべき現物、すなわち「不足していると売り注文が残高不足で失敗する」分だけを対象にする。
手動保有(Manual)は判定に含めない。
*/
func (h ExpectedHolding) AlertTarget() float64 {
	return h.Bot + h.Naked
}

/*
GetExpectedHoldings はDB上の想定保有量を product_code ごとに返す。

手動保有分は remarks に RemarkManualHold を含むかどうかで識別し、
Bot / Naked の集計からは必ず除外する（二重計上を防ぐため）。
いずれも集約(SUM)クエリであり全件取得は行わない。
*/
func GetExpectedHoldings() (map[string]ExpectedHolding, error) {
	// kind ごとの集計を1クエリにまとめる。sell_orders / buy_orders の状態は以下の意味を持つ:
	//   sell_orders UNFILLED           : 売り注文が生きている＝現物を保有している
	//   buy_orders  FILLED             : 買いが約定し、まだ売り注文を出していない＝現物を保有している
	//   buy_orders  FILLED(SELL ORDER PLACED) かつ 生存する売り注文が無い : 裸の保有
	baseQuery := `SELECT 'bot' AS kind, product_code, COALESCE(SUM(size), 0) AS total
			FROM sell_orders
			WHERE status = 'UNFILLED' AND order_id <> '' AND (remarks IS NULL OR remarks NOT LIKE %[1]s)
			GROUP BY product_code
		UNION ALL
		SELECT 'bot', product_code, COALESCE(SUM(size), 0)
			FROM buy_orders
			WHERE status = 'FILLED' AND order_id <> '' AND (remarks IS NULL OR remarks NOT LIKE %[1]s)
			GROUP BY product_code
		UNION ALL
		SELECT 'naked', b.product_code, COALESCE(SUM(b.size), 0)
			FROM buy_orders b
			WHERE b.status = 'FILLED(SELL ORDER PLACED)' AND b.order_id <> ''
			  AND (b.remarks IS NULL OR b.remarks NOT LIKE %[1]s)
			  AND NOT EXISTS (
			    SELECT 1 FROM sell_orders s
			    WHERE s.parentid = b.order_id AND s.status IN ('UNFILLED', 'FILLED')
			  )
			GROUP BY b.product_code
		UNION ALL
		SELECT 'manual', product_code, COALESCE(SUM(size), 0)
			FROM buy_orders
			WHERE order_id <> '' AND remarks LIKE %[1]s
			  AND status IN ('FILLED', 'FILLED(SELL ORDER PLACED)')
			GROUP BY product_code`

	var query string
	var args []any
	if database.CurrentDriver() == "postgres" {
		query = fmt.Sprintf(baseQuery, "$1")
		args = []any{manualHoldLikePattern}
	} else {
		query = fmt.Sprintf(baseQuery, "?")
		args = []any{manualHoldLikePattern, manualHoldLikePattern, manualHoldLikePattern, manualHoldLikePattern}
	}

	rows, err := AppDB.Query(query, args...)
	if err != nil {
		log.Printf("[ERROR] GetExpectedHoldings err:%v", err)
		return nil, err
	}
	defer rows.Close()

	holdings := make(map[string]ExpectedHolding)
	for rows.Next() {
		var (
			kind        string
			productCode sql.NullString
			total       sql.NullFloat64
		)
		if err := rows.Scan(&kind, &productCode, &total); err != nil {
			return nil, err
		}
		if productCode.String == "" {
			continue
		}
		holding := holdings[productCode.String]
		switch kind {
		case "bot":
			holding.Bot += total.Float64
		case "naked":
			holding.Naked += total.Float64
		case "manual":
			holding.Manual += total.Float64
		}
		holdings[productCode.String] = holding
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for productCode, holding := range holdings {
		log.Printf("GetExpectedHoldings product_code:%s bot:%v naked:%v manual:%v",
			productCode, holding.Bot, holding.Naked, holding.Manual)
	}
	return holdings, nil
}
