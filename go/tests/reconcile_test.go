package tests

import (
	"testing"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/enums"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

// insertReconcileBuyOrder はリコンサイル検証用の買い注文を remarks / strategy / timestamp 付きで登録する。
func insertReconcileBuyOrder(t *testing.T, orderID, productCode string, price, size float64,
	status string, strategy int, remarks *string, timestamp time.Time) {
	t.Helper()
	_, err := models.AppDB.Exec(
		`INSERT INTO buy_orders (order_id, product_code, side, price, size, exchange, status, strategy, remarks, timestamp)
		 VALUES ($1,$2,'BUY',$3,$4,'bitflyer',$5,$6,$7,$8)`,
		orderID, productCode, price, size, status, strategy, remarks, timestamp)
	if err != nil {
		t.Fatalf("insert buy order failed: %v", err)
	}
}

/*
TestGetExpectedHoldings は残高突合の基礎になる想定保有量の集計を検証する。

手動保有([MANUAL_HOLD])を除外できているか、売り注文が消えた「裸の保有」を拾えているか、
売り注文が生きているポジションを買い注文側と二重計上していないかは、
誤るとリコンサイルが毎日誤検知するため（あるいは本物の乖離を見逃すため）テストで担保する。
*/
func TestGetExpectedHoldings(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	now := time.Now().UTC()
	manual := " [MANUAL_HOLD]"

	// (1) 売り注文が生きているポジション: sell_orders(UNFILLED) 側で 0.001 を計上する。
	//     親の buy_orders は FILLED(SELL ORDER PLACED) なので二重計上されない
	insertReconcileBuyOrder(t, "B-OPEN", "BTC_JPY", 5000000, 0.001, "FILLED(SELL ORDER PLACED)", enums.StrategyLTP99, nil, now)
	insertTestSellOrder(t, "B-OPEN", "S-OPEN", "BTC_JPY", 5100000, 0.001, "UNFILLED")

	// (2) 約定済みでまだ売り注文を出していない: buy_orders(FILLED) 側で 0.002 を計上する
	insertReconcileBuyOrder(t, "B-FILLED", "BTC_JPY", 5000000, 0.002, "FILLED", enums.StrategyLTP98, nil, now)

	// (3) 裸の保有: 売り注文がCANCELLED（失効・ローリング失敗）で現物だけが残っている 0.004
	insertReconcileBuyOrder(t, "B-NAKED", "BTC_JPY", 5000000, 0.004, "FILLED(SELL ORDER PLACED)", enums.StrategyLTP97, nil, now)
	insertTestSellOrder(t, "B-NAKED", "S-NAKED", "BTC_JPY", 5100000, 0.004, "CANCELLED")

	// (4) 手動保有: [MANUAL_HOLD] 付き。bot / naked には含めず manual に 0.008 を計上する
	insertReconcileBuyOrder(t, "B-MANUAL", "BTC_JPY", 5000000, 0.008, "FILLED(SELL ORDER PLACED)", enums.StrategyLTP99, &manual, now)
	insertTestSellOrder(t, "B-MANUAL", "S-MANUAL", "BTC_JPY", 5100000, 0.008, "CANCELLED")

	// (5) 売却済み: 売り注文がFILLED＝現物は残っていないので想定保有量に含めない
	insertReconcileBuyOrder(t, "B-SOLD", "BTC_JPY", 5000000, 0.016, "FILLED(SELL ORDER PLACED)", enums.StrategyLTP99, nil, now)
	insertTestSellOrder(t, "B-SOLD", "S-SOLD", "BTC_JPY", 5100000, 0.016, "FILLED")

	// (6) 未約定の買い注文: 現物はまだ保有していない
	insertReconcileBuyOrder(t, "B-UNFILLED", "BTC_JPY", 4000000, 0.032, "UNFILLED", enums.StrategyLTP99, nil, now)

	// (7) 別通貨ペア: ETH_JPY が product_code ごとに分けて集計されることの確認
	insertReconcileBuyOrder(t, "B-ETH", "ETH_JPY", 400000, 0.05, "FILLED", enums.StrategyLTP99, nil, now)

	holdings, err := models.GetExpectedHoldings()
	if err != nil {
		t.Fatalf("GetExpectedHoldings failed: %v", err)
	}

	btc := holdings["BTC_JPY"]
	if btc.Bot != 0.003 {
		t.Errorf("BTC bot holding: got %v, want 0.003", btc.Bot)
	}
	if btc.Naked != 0.004 {
		t.Errorf("BTC naked holding: got %v, want 0.004", btc.Naked)
	}
	if btc.Manual != 0.008 {
		t.Errorf("BTC manual holding: got %v, want 0.008", btc.Manual)
	}
	if btc.Total() != 0.015 {
		t.Errorf("BTC total holding: got %v, want 0.015", btc.Total())
	}
	// 乖離判定に使うのは Bot + Naked のみ。手動保有(0.008)は含めない
	if btc.AlertTarget() != 0.007 {
		t.Errorf("BTC alert target: got %v, want 0.007 (manual holding must be excluded)", btc.AlertTarget())
	}

	eth := holdings["ETH_JPY"]
	if eth.Bot != 0.05 || eth.Naked != 0 || eth.Manual != 0 {
		t.Errorf("ETH holding: got bot=%v naked=%v manual=%v, want bot=0.05 naked=0 manual=0", eth.Bot, eth.Naked, eth.Manual)
	}
	if eth.AlertTarget() != 0.05 {
		t.Errorf("ETH alert target: got %v, want 0.05", eth.AlertTarget())
	}
}

// TestGetUnfilledOrderIDs は注文突合に使う未約定 order_id の抽出を検証する。
func TestGetUnfilledOrderIDs(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	now := time.Now().UTC()
	insertReconcileBuyOrder(t, "B-1", "BTC_JPY", 5000000, 0.001, "UNFILLED", enums.StrategyLTP99, nil, now)
	insertReconcileBuyOrder(t, "B-2", "BTC_JPY", 5000000, 0.001, "FILLED", enums.StrategyLTP99, nil, now)
	insertReconcileBuyOrder(t, "B-3", "ETH_JPY", 400000, 0.01, "UNFILLED", enums.StrategyLTP99, nil, now)
	insertTestSellOrder(t, "B-2", "S-1", "BTC_JPY", 5100000, 0.001, "UNFILLED")

	buyIDs, truncated, err := models.GetUnfilledOrderIDs(models.TableBuyOrders, "BTC_JPY", 100)
	if err != nil {
		t.Fatalf("GetUnfilledOrderIDs(buy) failed: %v", err)
	}
	if len(buyIDs) != 1 || buyIDs[0] != "B-1" {
		t.Errorf("buy unfilled ids: got %v, want [B-1]", buyIDs)
	}
	if truncated {
		t.Errorf("上限に達していないため truncated=false の想定")
	}

	sellIDs, truncated, err := models.GetUnfilledOrderIDs(models.TableSellOrders, "BTC_JPY", 100)
	if err != nil {
		t.Fatalf("GetUnfilledOrderIDs(sell) failed: %v", err)
	}
	if len(sellIDs) != 1 || sellIDs[0] != "S-1" {
		t.Errorf("sell unfilled ids: got %v, want [S-1]", sellIDs)
	}
	if truncated {
		t.Errorf("上限に達していないため truncated=false の想定")
	}
}

/*
F28 のテスト。

limit による打ち切りは reconcileJob の突合前提が崩れている状態だが、以前は
ローカルログに [ERROR] を出すだけで呼び出し元に伝わらず、突合結果が実態とずれたまま
「乖離なし」と通知されうる状態だった。打ち切りを戻り値で伝えられることを検証する。
*/
func TestGetUnfilledOrderIDsReportsTruncation(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	now := time.Now().UTC()
	for _, orderID := range []string{"T-1", "T-2", "T-3"} {
		insertReconcileBuyOrder(t, orderID, "BTC_JPY", 5000000, 0.001, "UNFILLED", enums.StrategyLTP99, nil, now)
	}

	// 上限に達した場合は truncated=true（取りこぼしが発生している可能性がある）
	ids, truncated, err := models.GetUnfilledOrderIDs(models.TableBuyOrders, "BTC_JPY", 2)
	if err != nil {
		t.Fatalf("GetUnfilledOrderIDs failed: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("取得件数: got %d, want 2", len(ids))
	}
	if !truncated {
		t.Errorf("上限に達したため truncated=true の想定")
	}

	// 件数ちょうどでも「これ以上あるか分からない」ため truncated=true に倒す
	_, truncated, err = models.GetUnfilledOrderIDs(models.TableBuyOrders, "BTC_JPY", 3)
	if err != nil {
		t.Fatalf("GetUnfilledOrderIDs failed: %v", err)
	}
	if !truncated {
		t.Errorf("件数と上限が一致する場合も truncated=true の想定")
	}

	// 上限に余裕があれば truncated=false（リグレッション）
	ids, truncated, err = models.GetUnfilledOrderIDs(models.TableBuyOrders, "BTC_JPY", 4)
	if err != nil {
		t.Fatalf("GetUnfilledOrderIDs failed: %v", err)
	}
	if len(ids) != 3 || truncated {
		t.Errorf("全件取得できた場合: got %d件 truncated=%v, want 3件 truncated=false", len(ids), truncated)
	}
}

// TestGetUnfilledOrdersWithRemark はローリング再試行待ちマーカーの抽出を検証する。
func TestGetUnfilledOrdersWithRemark(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	now := time.Now().UTC()
	pending := " " + models.RemarkRolloverPending
	insertRolloverSellOrder(t, "P1", "S-PENDING", "BTC_JPY", 5100000, 0.001, "UNFILLED", &pending, nil, now)
	insertRolloverSellOrder(t, "P2", "S-NORMAL", "BTC_JPY", 5100000, 0.001, "UNFILLED", nil, nil, now)
	// CANCELLED はマーカーが付いていても対象外（手動保有レコードを拾わないことの確認）
	insertRolloverSellOrder(t, "P3", "S-CANCELLED", "BTC_JPY", 5100000, 0.001, "CANCELLED", &pending, nil, now)

	records, err := models.GetUnfilledOrdersWithRemark(models.TableSellOrders, models.RemarkRolloverPending, 50)
	if err != nil {
		t.Fatalf("GetUnfilledOrdersWithRemark failed: %v", err)
	}
	if len(records) != 1 || records[0].OrderID != "S-PENDING" {
		t.Fatalf("rollover pending records: got %v, want [S-PENDING]", records)
	}
	if records[0].ParentID != "P1" {
		t.Errorf("parentid: got %s, want P1", records[0].ParentID)
	}
}

// TestGetRecentBuyOrders は発注ゼロ検知の基礎になる直近買い注文の取得順とボット判定を検証する。
func TestGetRecentBuyOrders(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	now := time.Now().UTC()
	insertReconcileBuyOrder(t, "B-OLD-BOT", "BTC_JPY", 5000000, 0.001, "UNFILLED", enums.StrategyLTP99, nil, now.AddDate(0, 0, -10))
	insertReconcileBuyOrder(t, "B-NEW-MANUAL", "BTC_JPY", 5000000, 0.001, "UNFILLED", enums.StrategyManual, nil, now)

	records, err := models.GetRecentBuyOrders(50)
	if err != nil {
		t.Fatalf("GetRecentBuyOrders failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("record count: got %d, want 2", len(records))
	}
	// 新しい順に返ること
	if records[0].OrderID != "B-NEW-MANUAL" {
		t.Errorf("first record: got %s, want B-NEW-MANUAL", records[0].OrderID)
	}
	// 手動取り込み(90001)はボット発注として数えない
	if enums.IsBotStrategy(records[0].Strategy) {
		t.Errorf("strategy %d should not be a bot strategy", records[0].Strategy)
	}
	if !enums.IsBotStrategy(records[1].Strategy) {
		t.Errorf("strategy %d should be a bot strategy", records[1].Strategy)
	}
}
