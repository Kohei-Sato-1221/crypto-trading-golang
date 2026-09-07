package tests

import (
	"testing"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/database"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

/*
F9 のテスト。

models 層のエラーがログ出力のみで呼び出し元に伝わらず、Slack にも届いていなかった。
とくに CheckFilledBuyOrders は失敗時に nil を返すため、placeSellOrder が
「売る対象なし」と解釈し、約定済み買い注文への売り注文発注が無言でスキップされていた。

「対象0件」と「読み取り失敗」を呼び出し元が区別できることを検証する。
*/

// 正常系の回帰確認: FILLED の買い注文だけが返り、error は nil であること。
func TestCheckFilledBuyOrdersReturnsRecords(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	insertTestBuyOrder(t, "B-FILLED", "BTC_JPY", 5000000, 0.001, models.OrderStatusFilled)
	insertTestBuyOrder(t, "B-UNFILLED", "BTC_JPY", 5000000, 0.001, models.OrderStatusUnfilled)
	insertTestBuyOrder(t, "B-PLACED", "BTC_JPY", 5000000, 0.001, models.OrderStatusFilledSellOrderPlaced)

	orders, err := models.CheckFilledBuyOrders()
	if err != nil {
		t.Fatalf("CheckFilledBuyOrders failed: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("FILLED の買い注文のみ返る想定: got %d件 %v", len(orders), orders)
	}
	if orders[0].OrderID != "B-FILLED" {
		t.Errorf("order_id: got %s, want B-FILLED", orders[0].OrderID)
	}
	// strategy の代入漏れがあると CalculateSellOrderPrice() の戦略分岐が働かない
	if orders[0].Strategy != 1 {
		t.Errorf("strategy: got %v, want 1", orders[0].Strategy)
	}
	if orders[0].Size != 0.001 || orders[0].ProductCode != "BTC_JPY" {
		t.Errorf("size/product_code: got %v/%s", orders[0].Size, orders[0].ProductCode)
	}
}

// 対象0件は error ではないこと（0件と失敗を取り違えないことの確認）。
func TestCheckFilledBuyOrdersEmptyIsNotError(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	orders, err := models.CheckFilledBuyOrders()
	if err != nil {
		t.Fatalf("0件でも error にはならない想定: %v", err)
	}
	if len(orders) != 0 {
		t.Fatalf("0件の想定: got %v", orders)
	}
}

// 読み取りに失敗した場合は error を返すこと（無言スキップしない）。
func TestCheckFilledBuyOrdersReturnsErrorOnQueryFailure(t *testing.T) {
	setupTestDB(t)
	// 接続を閉じてクエリを失敗させる
	if err := database.Current.Close(); err != nil {
		t.Fatalf("failed to close test db: %v", err)
	}

	orders, err := models.CheckFilledBuyOrders()
	if err == nil {
		t.Fatalf("DB読み取り失敗時は error を返す想定だが nil だった (orders:%v)", orders)
	}
	if orders != nil {
		t.Errorf("失敗時はレコードを返さない想定: got %v", orders)
	}

	// 後続テストのために接続を張り直す
	setupTestDB(t)
	teardownTestDB(t)
}

/*
SyncBuyOrders は1件の失敗で全体を止めず、発生した失敗をまとめて返す。
呼び出し元(syncBuyOrders)がそれをSlackへ通知する。
*/

// 正常系の回帰確認: 新規注文が取り込まれ、失敗は返らないこと。
func TestSyncBuyOrdersInsertsWithoutFailures(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	now := time.Now().UTC().Truncate(time.Second)
	expire := now.Add(7 * 24 * time.Hour)
	events := []models.OrderEvent{
		{
			OrderID:     "SYNC-1",
			Time:        now,
			ProductCode: "ETH_JPY",
			Side:        "BUY",
			Price:       512345,
			Size:        0.02,
			Exchange:    "bitflyer",
			Status:      models.OrderStatusUnfilled,
			Strategy:    11,
			ExpireDate:  &expire,
		},
	}

	failures := models.SyncBuyOrders(&events)
	if len(failures) != 0 {
		t.Fatalf("正常系では失敗0件の想定: got %v", failures)
	}

	var count int
	var storedExpire time.Time
	if err := getDB(t).QueryRow(
		`SELECT COUNT(*), MAX(expire_date) FROM buy_orders WHERE order_id = $1`, "SYNC-1").
		Scan(&count, &storedExpire); err != nil {
		t.Fatalf("failed to read synced order: %v", err)
	}
	if count != 1 {
		t.Fatalf("取り込み件数: got %d, want 1", count)
	}
	// DBへはUTCで保存する
	if !storedExpire.UTC().Equal(expire) {
		t.Errorf("expire_date(UTC): got %s, want %s",
			storedExpire.UTC().Format(time.RFC3339), expire.Format(time.RFC3339))
	}
}

// 取り込みに失敗した場合、コンテキスト付きの失敗が返ること（ログ止まりにしない）。
func TestSyncBuyOrdersReturnsFailures(t *testing.T) {
	setupTestDB(t)
	if err := database.Current.Close(); err != nil {
		t.Fatalf("failed to close test db: %v", err)
	}

	now := time.Now().UTC()
	events := []models.OrderEvent{
		{
			OrderID:     "SYNC-NG",
			Time:        now,
			ProductCode: "BTC_JPY",
			Side:        "BUY",
			Price:       5000000,
			Size:        0.001,
			Exchange:    "bitflyer",
			Status:      models.OrderStatusUnfilled,
			Strategy:    1,
		},
	}

	failures := models.SyncBuyOrders(&events)
	if len(failures) != 1 {
		t.Fatalf("失敗1件が返る想定: got %d件 %v", len(failures), failures)
	}
	failure := failures[0]
	if failure.OrderID != "SYNC-NG" || failure.ProductCode != "BTC_JPY" ||
		failure.Price != 5000000 || failure.Size != 0.001 || failure.Strategy != 1 {
		t.Errorf("通知に必要なコンテキストが欠けている: %+v", failure)
	}
	if failure.Err == nil {
		t.Errorf("原因の error が保持されていない: %+v", failure)
	}

	setupTestDB(t)
	teardownTestDB(t)
}
