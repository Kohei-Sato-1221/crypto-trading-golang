package tests

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

/*
売り注文の部分約定が起きたときの手動対応手順（ルート CLAUDE.md
「売り注文の部分約定が起きたときの対応」）が、実際に損益を正しくすることを
実クエリ（models.GetResults / getResultsPostgres）で検証する。

背景:
損益は sum((a.price * a.size) - (b.price * b.size)) を
a=sell(FILLED) / b=buy / a.parentid = b.order_id で集計している。
部分約定した売り注文をローリングすると旧レコードは CANCELLED になり集計から外れるため、
約定済み数量ぶんの売却額が丸ごと落ちて損益が過小に出る（安全側だが不正確）。
F27（買い注文側の数量按分による恒久対応）はスコープ外と判断されたため、
発生時は手作業でレコードを補う運用とした。その手順の正しさを固定するのがこのテスト。

**手順が壊れたときに気づけるようにするためのテストであり、
CLAUDE.md に書いてある SQL と同じ形の操作をここで再現している。**
手順を変えるときは CLAUDE.md とこのテストの両方を更新すること。
*/

const (
	pfBuyOrderID    = "PF-BUY-1"
	pfOldSellID     = "PF-SELL-OLD"
	pfNewSellID     = "PF-SELL-NEW"
	pfBuyPrice      = 500000.0 // P_b: 買値
	pfPartialPrice  = 511000.0 // P_s1: 部分約定の平均約定価格(average_price)
	pfFinalPrice    = 512345.0 // P_s2: 残数量が後日約定した価格
	pfOriginalSize  = 0.03     // 元の数量
	pfExecutedSize  = 0.01     // 部分約定した数量
	pfRemainingSize = 0.02     // 残数量
)

// 損益レポートは手数料率 0.9989 を掛けた値を出力する
const pfFeeFactor = 0.9989

/*
totalProfitFromResults は models.GetResults() の出力から Total 行の profit を取り出す。

出力形式:

	【bitflyer 自動売買 収益】
	date / profit / count / ppt
	Total / 356.51 / 2 / 178.26
*/
func totalProfitFromResults(t *testing.T) float64 {
	t.Helper()

	result, err := models.GetResults()
	if err != nil {
		t.Fatalf("GetResults() failed: %v", err)
	}
	for _, line := range strings.Split(result, "\n") {
		if !strings.HasPrefix(line, "Total /") {
			continue
		}
		fields := strings.Split(line, " / ")
		if len(fields) < 2 {
			t.Fatalf("Total 行を解釈できない: %q", line)
		}
		profit, parseErr := strconv.ParseFloat(strings.TrimSpace(fields[1]), 64)
		if parseErr != nil {
			t.Fatalf("profit を数値に変換できない: %q (%v)", line, parseErr)
		}
		return profit
	}
	t.Fatalf("Total 行が見つからない:\n%s", result)
	return 0
}

func assertProfit(t *testing.T, label string, got, want float64) {
	t.Helper()
	if diff := got - want; diff > 0.05 || diff < -0.05 {
		t.Errorf("%s: profit = %v, want %v", label, got, want)
	}
}

// setSellUpdatetime は集計対象になるよう売り注文の updatetime を今日(UTC)に寄せる。
func setSellUpdatetime(t *testing.T, orderID string) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := models.AppDB.Exec(
		"UPDATE sell_orders SET updatetime = $1 WHERE order_id = $2", now, orderID); err != nil {
		t.Fatalf("failed to update sell order updatetime: %v", err)
	}
}

/*
setupPartialFillScenario は「部分約定 → ローリング → 残数量が後日約定」の状態を再現する。

  - buy_orders:  size=0.03 @500000（発注時のまま。按分されていない）
  - sell_orders: 旧レコード=CANCELLED（ローリングで巻き直された。size は残数量へ補正済み）
    新レコード=FILLED   size=0.02 @512345
  - 部分約定した 0.01 @511000 はどこにも記録されていない ← これが過小計上の原因
*/
func setupPartialFillScenario(t *testing.T) {
	t.Helper()

	insertTestBuyOrder(t, pfBuyOrderID, "ETH_JPY", pfBuyPrice, pfOriginalSize, models.OrderStatusFilledSellOrderPlaced)
	// 旧売り注文: 部分約定を検出して size を残数量へ補正したうえでローリングされ CANCELLED になった
	insertTestSellOrder(t, pfBuyOrderID, pfOldSellID, "ETH_JPY", pfFinalPrice, pfRemainingSize, models.OrderStatusCancelled)
	// 巻き直し後の売り注文が後日約定した
	insertTestSellOrder(t, pfBuyOrderID, pfNewSellID, "ETH_JPY", pfFinalPrice, pfRemainingSize, models.OrderStatusFilled)
	setSellUpdatetime(t, pfNewSellID)
}

/*
現状（手動対応をしていない状態）では、部分約定ぶんの売却額が丸ごと欠落すること。

真の損益  = (P_s1 - P_b) * 0.01 + (P_s2 - P_b) * 0.02
現在の記録 = (P_s2 * 0.02) - (P_b * 0.03)
差分      = P_s1 * 0.01（＝部分約定ぶんの売却総額）
*/
func TestPartialFillProfitIsUnderstatedWithoutManualFix(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	setupPartialFillScenario(t)

	want := (pfFinalPrice*pfRemainingSize - pfBuyPrice*pfOriginalSize) * pfFeeFactor
	got := totalProfitFromResults(t)
	assertProfit(t, "手動対応なし", got, want)

	trueProfit := ((pfPartialPrice-pfBuyPrice)*pfExecutedSize + (pfFinalPrice-pfBuyPrice)*pfRemainingSize) * pfFeeFactor
	if got >= trueProfit {
		t.Errorf("過小計上になっていない: got=%v trueProfit=%v", got, trueProfit)
	}
	t.Logf("手動対応なし: %v (真の損益 %v / 不足額 %v)", got, trueProfit, trueProfit-got)
}

/*
アンチパターンの固定: 買い注文を分割せずに売り注文だけ INSERT すると、
同一 parentid に FILLED の売りが2行並び **買いコスト(b.price * b.size)が2回引かれて**
かえって悪化すること。CLAUDE.md がこのやり方を禁じている根拠。
*/
func TestPartialFillNaiveSellInsertMakesItWorse(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	setupPartialFillScenario(t)
	before := totalProfitFromResults(t)

	// アンチパターン: parentid を分けずに部分約定ぶんの売りだけ足す
	insertTestSellOrder(t, pfBuyOrderID, pfOldSellID+"-PARTIAL", "ETH_JPY",
		pfPartialPrice, pfExecutedSize, models.OrderStatusFilled)
	setSellUpdatetime(t, pfOldSellID+"-PARTIAL")

	want := (pfFinalPrice*pfRemainingSize - pfBuyPrice*pfOriginalSize +
		pfPartialPrice*pfExecutedSize - pfBuyPrice*pfOriginalSize) * pfFeeFactor
	got := totalProfitFromResults(t)
	assertProfit(t, "単純INSERT(アンチパターン)", got, want)

	if got >= before {
		t.Errorf("買いコストの二重計上が再現できていない: before=%v after=%v", before, got)
	}
	t.Logf("単純INSERT(アンチパターン): %v （対応前 %v より悪化）", got, before)
}

/*
CLAUDE.md の手順（買い注文を按分してから売り注文を足す）で損益が正しくなること。

 1. 既存の買い注文の size を残数量へ更新する          (0.03 -> 0.02)
 2. 部分約定ぶんの買い注文を派生IDで INSERT する       (size=0.01 price=P_b)
 3. 部分約定ぶんの売り注文を手順2の買いに紐づけて INSERT (size=0.01 price=P_s1=average_price)
*/
func TestPartialFillManualFixRestoresCorrectProfit(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	setupPartialFillScenario(t)

	partialBuyID := pfBuyOrderID + "-PARTIAL"
	partialSellID := pfOldSellID + "-PARTIAL"

	// 手順1: 既存の買い注文を残数量へ按分する
	if _, err := models.AppDB.Exec(
		`UPDATE buy_orders SET size = $1, remarks = COALESCE(remarks, '') || $2 WHERE order_id = $3`,
		pfRemainingSize, " / partial fill manual fix: size -> 0.02", pfBuyOrderID); err != nil {
		t.Fatalf("手順1(買い注文の按分)に失敗: %v", err)
	}

	// 手順2: 部分約定ぶんの買い注文を派生IDで作る（order_id は UNIQUE 制約があるため元IDは使えない）
	if _, err := models.AppDB.Exec(
		`INSERT INTO buy_orders (order_id, product_code, side, price, size, exchange, status, strategy, remarks, timestamp)
		 SELECT $1, product_code, side, price, $2, exchange, status, strategy,
		        COALESCE(remarks, '') || ' / partial fill manual fix: split from ' || order_id, timestamp
		 FROM buy_orders WHERE order_id = $3`,
		partialBuyID, pfExecutedSize, pfBuyOrderID); err != nil {
		t.Fatalf("手順2(部分約定ぶんの買い注文INSERT)に失敗: %v", err)
	}

	// 手順3: 部分約定ぶんの売り注文を手順2の買いに紐づけて作る（price は average_price）
	if _, err := models.AppDB.Exec(
		`INSERT INTO sell_orders (parentid, order_id, product_code, side, price, size, exchange, status, remarks, updatetime)
		 VALUES ($1, $2, 'ETH_JPY', 'SELL', $3, $4, 'bitflyer', $5, 'partial fill manual fix', $6)`,
		partialBuyID, partialSellID, pfPartialPrice, pfExecutedSize,
		models.OrderStatusFilled, time.Now().UTC()); err != nil {
		t.Fatalf("手順3(部分約定ぶんの売り注文INSERT)に失敗: %v", err)
	}

	want := ((pfPartialPrice-pfBuyPrice)*pfExecutedSize + (pfFinalPrice-pfBuyPrice)*pfRemainingSize) * pfFeeFactor
	got := totalProfitFromResults(t)
	assertProfit(t, "手動対応あり", got, want)
	t.Logf("手動対応あり: %v (期待値 %v)", got, want)

	// 検証クエリ: 買い数量の合計が元の数量から変わっていないこと（分割であって水増しではない）
	var totalBuySize float64
	if err := models.AppDB.QueryRow(
		`SELECT COALESCE(SUM(size), 0) FROM buy_orders WHERE order_id IN ($1, $2)`,
		pfBuyOrderID, partialBuyID).Scan(&totalBuySize); err != nil {
		t.Fatalf("買い数量の検算に失敗: %v", err)
	}
	if diff := totalBuySize - pfOriginalSize; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("買い数量の合計が元と一致しない: %v want %v", totalBuySize, pfOriginalSize)
	}

	// 検証クエリ: 1つの買い注文に FILLED の売りが2行以上ぶら下がっていないこと（二重計上の検知）
	var dupCount int
	if err := models.AppDB.QueryRow(
		`SELECT COUNT(*) FROM (
		   SELECT parentid FROM sell_orders WHERE status = 'FILLED' GROUP BY parentid HAVING COUNT(*) > 1
		 ) d`).Scan(&dupCount); err != nil {
		t.Fatalf("二重計上の検算に失敗: %v", err)
	}
	if dupCount != 0 {
		t.Errorf("同一 parentid に FILLED の売りが複数ある（買いコストが二重計上される）: %d件", dupCount)
	}
}

/*
手動対応で追加したレコードが、残高照合(GetExpectedHoldings)を狂わせないこと。

派生した買い注文は status='FILLED(SELL ORDER PLACED)' かつ FILLED の売りが
紐づくため「裸の保有(naked)」にも「bot」にも計上されない。
ここが崩れると残高乖離アラートが誤発火する。
*/
func TestPartialFillManualFixDoesNotBreakExpectedHoldings(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	setupPartialFillScenario(t)

	before, err := models.GetExpectedHoldings()
	if err != nil {
		t.Fatalf("GetExpectedHoldings() failed: %v", err)
	}

	partialBuyID := pfBuyOrderID + "-PARTIAL"
	partialSellID := pfOldSellID + "-PARTIAL"
	if _, err := models.AppDB.Exec(`UPDATE buy_orders SET size = $1 WHERE order_id = $2`,
		pfRemainingSize, pfBuyOrderID); err != nil {
		t.Fatalf("手順1に失敗: %v", err)
	}
	insertTestBuyOrder(t, partialBuyID, "ETH_JPY", pfBuyPrice, pfExecutedSize, models.OrderStatusFilledSellOrderPlaced)
	insertTestSellOrder(t, partialBuyID, partialSellID, "ETH_JPY", pfPartialPrice, pfExecutedSize, models.OrderStatusFilled)

	after, err := models.GetExpectedHoldings()
	if err != nil {
		t.Fatalf("GetExpectedHoldings() failed: %v", err)
	}

	if after["ETH_JPY"].AlertTarget() != before["ETH_JPY"].AlertTarget() {
		t.Errorf("手動対応で想定保有量が変わってしまった: before=%v after=%v",
			before["ETH_JPY"], after["ETH_JPY"])
	}
}
