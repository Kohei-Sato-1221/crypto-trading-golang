package tests

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

/*
F22 の回帰テスト。

MarkOrderCancelledWithRemark だけが RowsAffected() のエラーを無視して nil を返しており、
「CANCELLEDにできた」と断定できないまま成功扱いになっていた。expireSweepJob は
失効処理を完了したものとして集計・通知するため、実際には UNFILLED のまま残った
レコードが誰にも拾われなくなる。RolloverSellOrder / UpdateOrderSizeWithRemark と
同じく、確認できない場合は失敗側へ倒す（sweep は通知して翌日再試行する）。
*/

// --- RowsAffected() が必ずエラーを返すダミードライバ ---

type rowsErrDriver struct{}

func (rowsErrDriver) Open(string) (driver.Conn, error) { return rowsErrConn{}, nil }

type rowsErrConn struct{}

func (rowsErrConn) Prepare(string) (driver.Stmt, error) { return rowsErrStmt{}, nil }
func (rowsErrConn) Close() error                        { return nil }
func (rowsErrConn) Begin() (driver.Tx, error)           { return nil, errors.New("begin is not supported") }

type rowsErrStmt struct{}

func (rowsErrStmt) Close() error  { return nil }
func (rowsErrStmt) NumInput() int { return -1 }
func (rowsErrStmt) Exec([]driver.Value) (driver.Result, error) {
	return rowsErrResult{}, nil
}
func (rowsErrStmt) Query([]driver.Value) (driver.Rows, error) {
	return nil, errors.New("query is not supported")
}

type rowsErrResult struct{}

func (rowsErrResult) LastInsertId() (int64, error) { return 0, errors.New("not supported") }
func (rowsErrResult) RowsAffected() (int64, error) {
	return 0, errors.New("rows affected is unavailable on this driver")
}

func init() {
	sql.Register("rowsafffectederr", rowsErrDriver{})
}

// 更新行数を確認できない場合はエラーを返すこと（成功扱いにしない）。
func TestMarkOrderCancelledWithRemarkFailsWhenRowsAffectedUnavailable(t *testing.T) {
	db, err := sql.Open("rowsafffectederr", "")
	if err != nil {
		t.Fatalf("failed to open stub db: %v", err)
	}
	defer db.Close()

	original := models.AppDB
	models.AppDB = db
	defer func() { models.AppDB = original }()

	err = models.MarkOrderCancelledWithRemark(models.TableSellOrders, "S-UNCONFIRMED", " / expired at test")
	if err == nil {
		t.Fatal("RowsAffected() が失敗しても nil を返している（sweep が失効処理を完了したものとして集計してしまう）")
	}
	// 通知に必要なコンテキスト（テーブル・OrderID）が含まれること
	for _, want := range []string{"sell_orders", "S-UNCONFIRMED"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("エラーに %q が含まれていない: %v", want, err)
		}
	}
}

// リグレッション: 正常に更新できた場合は nil を返し、statusとremarksが更新されること。
func TestMarkOrderCancelledWithRemarkUpdatesUnfilledRow(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	now := time.Now().UTC()
	insertRolloverSellOrder(t, "P-1", "S-SWEEP", "BTC_JPY", 5100000, 0.001, "UNFILLED", nil, nil, now)

	remark := " / expired at test (auto sweep, method=A)"
	if err := models.MarkOrderCancelledWithRemark(models.TableSellOrders, "S-SWEEP", remark); err != nil {
		t.Fatalf("MarkOrderCancelledWithRemark failed: %v", err)
	}

	var status, remarks string
	if err := getDB(t).QueryRow(
		`SELECT status, COALESCE(remarks, '') FROM sell_orders WHERE order_id = $1`, "S-SWEEP").
		Scan(&status, &remarks); err != nil {
		t.Fatalf("failed to read record: %v", err)
	}
	if status != models.OrderStatusCancelled {
		t.Errorf("status: got %s, want %s", status, models.OrderStatusCancelled)
	}
	if !strings.Contains(remarks, "auto sweep") {
		t.Errorf("remarks に経緯が追記されていない: %s", remarks)
	}
}

// リグレッション: 対象行が無い（他ジョブが先に更新した）場合はエラーにしないこと。
func TestMarkOrderCancelledWithRemarkNoRowIsNotError(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	now := time.Now().UTC()
	// 既に FILLED。UNFILLED の行だけを対象にするので更新行数は0になる
	insertRolloverSellOrder(t, "P-2", "S-FILLED", "BTC_JPY", 5100000, 0.001, "FILLED", nil, nil, now)

	if err := models.MarkOrderCancelledWithRemark(models.TableSellOrders, "S-FILLED", " / expired at test"); err != nil {
		t.Fatalf("更新行数0は正常な競合であり error にしない想定: %v", err)
	}

	var status string
	if err := getDB(t).QueryRow(
		`SELECT status FROM sell_orders WHERE order_id = $1`, "S-FILLED").Scan(&status); err != nil {
		t.Fatalf("failed to read record: %v", err)
	}
	if status != "FILLED" {
		t.Errorf("他ジョブが更新した status を上書きしている: got %s, want FILLED", status)
	}
}
