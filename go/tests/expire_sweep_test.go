package tests

import (
	"testing"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

/*
F2 の回帰テスト。

[ROLLOVER_PENDING] 付きのレコードが expireSweepJob からも rolloverSellOrderJob からも
外れて恒久的に UNFILLED のまま滞留しないことを確認する。
滞留するとスロット(max_sell_orders)を食い潰し、design-dock §6.2 の
「売り注文が失効しました」通知も原理的に発火しなくなる。
*/

// rolloverRetryAfter は expireSweepRolloverRetryAfter() と同じ境界（既定 27+3=30日）。
func rolloverRetryAfter(now time.Time) time.Time {
	return now.UTC().AddDate(0, 0, -30)
}

// sweepIDs は方式Aの抽出結果を order_id の集合にする。
func sweepIDs(t *testing.T, table models.OrderTable, now time.Time) map[string]bool {
	t.Helper()
	records, err := models.GetExpiredUnfilledOrders(table, now, 10*time.Minute, 200)
	if err != nil {
		t.Fatalf("GetExpiredUnfilledOrders failed: %v", err)
	}
	got := map[string]bool{}
	for _, r := range records {
		got[r.OrderID] = true
	}
	return got
}

// sweepIDsWithoutExpireDate は方式Bの抽出結果を order_id の集合にする。
func sweepIDsWithoutExpireDate(t *testing.T, table models.OrderTable, productCode string, now time.Time) map[string]bool {
	t.Helper()
	records, err := models.GetUnfilledOrdersWithoutExpireDate(table, productCode, rolloverRetryAfter(now), 200)
	if err != nil {
		t.Fatalf("GetUnfilledOrdersWithoutExpireDate failed: %v", err)
	}
	got := map[string]bool{}
	for _, r := range records {
		got[r.OrderID] = true
	}
	return got
}

// rolloverIDs はローリング対象の order_id 集合を返す。
func rolloverIDs(t *testing.T, now time.Time) map[string]bool {
	t.Helper()
	records, err := models.GetSellOrdersToRollover(now, 3, 27, 200)
	if err != nil {
		t.Fatalf("GetSellOrdersToRollover failed: %v", err)
	}
	got := map[string]bool{}
	for _, r := range records {
		got[r.OrderID] = true
	}
	return got
}

// 方式A: 期限を過ぎた [ROLLOVER_PENDING] レコードが sweep の対象になること（F2 の本丸）。
func TestExpiredRolloverPendingIsSweptByMethodA(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	now := time.Now().UTC()
	pending := " " + models.RemarkRolloverPending
	expPast := now.AddDate(0, 0, -1) // 期限切れ
	expSoon := now.AddDate(0, 0, 2)  // 期限まで2日 = ローリングの窓の中

	// 修正対象: キャンセル成功・再発注失敗のまま期限を過ぎたレコード
	insertRolloverSellOrder(t, "P1", "S-PENDING-EXPIRED", "BTC_JPY", 5000000, 0.001, "UNFILLED", &pending, &expPast, now.AddDate(0, 0, -31))
	// 回帰: マーカーが無い通常の期限切れレコードは従来どおり sweep の対象
	insertRolloverSellOrder(t, "P2", "S-PLAIN-EXPIRED", "BTC_JPY", 5000000, 0.001, "UNFILLED", nil, &expPast, now.AddDate(0, 0, -31))
	// 回帰: 再試行中(期限内)の [ROLLOVER_PENDING] は sweep に横取りされずローリングの担当のまま
	insertRolloverSellOrder(t, "P3", "S-PENDING-ACTIVE", "BTC_JPY", 5000000, 0.001, "UNFILLED", &pending, &expSoon, now.AddDate(0, 0, -28))
	// 回帰: UNFILLED でないレコード（手動保有・約定済み）は構造的に対象外
	manual := " " + models.RemarkManualHold
	insertRolloverSellOrder(t, "P4", "S-MANUAL", "BTC_JPY", 5000000, 0.001, "CANCELLED", &manual, &expPast, now.AddDate(0, 0, -100))
	insertRolloverSellOrder(t, "P5", "S-FILLED", "BTC_JPY", 5000000, 0.001, "FILLED", nil, &expPast, now.AddDate(0, 0, -31))

	swept := sweepIDs(t, models.TableSellOrders, now)
	rolled := rolloverIDs(t, now)

	if !swept["S-PENDING-EXPIRED"] {
		t.Errorf("期限切れの %s レコードは sweep の対象でなければならない（恒久滞留の原因）: swept=%v",
			models.RemarkRolloverPending, swept)
	}
	if rolled["S-PENDING-EXPIRED"] {
		t.Errorf("期限切れレコードはローリングの対象にならない（キャンセルAPIが失敗するため）")
	}
	if !swept["S-PLAIN-EXPIRED"] {
		t.Errorf("通常の期限切れレコードが sweep の対象から外れた（回帰）: swept=%v", swept)
	}
	if swept["S-PENDING-ACTIVE"] {
		t.Errorf("再試行中の %s レコードを sweep が横取りしてはならない", models.RemarkRolloverPending)
	}
	if !rolled["S-PENDING-ACTIVE"] {
		t.Errorf("再試行中の %s レコードはローリングの対象でなければならない: rolled=%v",
			models.RemarkRolloverPending, rolled)
	}
	for _, id := range []string{"S-MANUAL", "S-FILLED"} {
		if swept[id] || rolled[id] {
			t.Errorf("UNFILLED でない %s がどちらかのジョブの対象になった", id)
		}
	}
}

// 方式B: expire_date が NULL の [ROLLOVER_PENDING] レコードの担当がローリングから sweep へ引き継がれること。
func TestRolloverPendingWithoutExpireDateIsHandedOverToSweep(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	now := time.Now().UTC()
	pending := " " + models.RemarkRolloverPending

	// ローリングの窓の内側(27〜30日) -> sweep からは外れる
	insertRolloverSellOrder(t, "P1", "S-PENDING-INWINDOW", "BTC_JPY", 5000000, 0.001, "UNFILLED", &pending, nil, now.AddDate(0, 0, -28))
	// ローリングの窓より古い(>30日) -> ローリングは諦めるので sweep が担当する
	insertRolloverSellOrder(t, "P2", "S-PENDING-TOOOLD", "BTC_JPY", 5000000, 0.001, "UNFILLED", &pending, nil, now.AddDate(0, 0, -40))
	// 回帰: マーカーが無いレコードは窓の内外にかかわらず方式Bの候補になる
	insertRolloverSellOrder(t, "P3", "S-PLAIN-INWINDOW", "BTC_JPY", 5000000, 0.001, "UNFILLED", nil, nil, now.AddDate(0, 0, -28))
	insertRolloverSellOrder(t, "P4", "S-PLAIN-TOOOLD", "BTC_JPY", 5000000, 0.001, "UNFILLED", nil, nil, now.AddDate(0, 0, -40))

	swept := sweepIDsWithoutExpireDate(t, models.TableSellOrders, "BTC_JPY", now)
	rolled := rolloverIDs(t, now)

	if swept["S-PENDING-INWINDOW"] {
		t.Errorf("再試行中の %s レコードを方式Bが横取りしてはならない", models.RemarkRolloverPending)
	}
	if !rolled["S-PENDING-INWINDOW"] {
		t.Errorf("窓の内側の %s レコードはローリングの対象でなければならない: rolled=%v",
			models.RemarkRolloverPending, rolled)
	}
	if !swept["S-PENDING-TOOOLD"] {
		t.Errorf("窓より古い %s レコードは方式Bの候補でなければならない（恒久滞留の原因）: swept=%v",
			models.RemarkRolloverPending, swept)
	}
	if rolled["S-PENDING-TOOOLD"] {
		t.Errorf("窓より古いレコードはローリングの対象にならない")
	}
	for _, id := range []string{"S-PLAIN-INWINDOW", "S-PLAIN-TOOOLD"} {
		if !swept[id] {
			t.Errorf("マーカーの無いレコード %s が方式Bの候補から外れた（回帰）: swept=%v", id, swept)
		}
	}
}

/*
どのジョブにも拾われないレコードが存在しないことを確認する。

F2 の本質は「sweep からも rollover からも外れる隙間」だったため、
代表的な状態をまとめて投入し、UNFILLED のレコードが必ずどちらかの担当になることを検証する。
*/
func TestNoUnfilledSellOrderFallsBetweenSweepAndRollover(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	now := time.Now().UTC()
	pending := " " + models.RemarkRolloverPending
	expPast := now.AddDate(0, 0, -1)
	expSoon := now.AddDate(0, 0, 2)

	cases := []struct {
		orderID   string
		remarks   *string
		expire    *time.Time
		timestamp time.Time
	}{
		{"C-PENDING-EXPIRED", &pending, &expPast, now.AddDate(0, 0, -31)},
		{"C-PENDING-SOON", &pending, &expSoon, now.AddDate(0, 0, -28)},
		{"C-PLAIN-EXPIRED", nil, &expPast, now.AddDate(0, 0, -31)},
		{"C-PLAIN-SOON", nil, &expSoon, now.AddDate(0, 0, -28)},
		{"C-PENDING-NULL-OLD", &pending, nil, now.AddDate(0, 0, -40)},
		{"C-PENDING-NULL-INWINDOW", &pending, nil, now.AddDate(0, 0, -28)},
		{"C-PLAIN-NULL-OLD", nil, nil, now.AddDate(0, 0, -40)},
	}
	for _, c := range cases {
		insertRolloverSellOrder(t, "P-"+c.orderID, c.orderID, "BTC_JPY", 5000000, 0.001, "UNFILLED",
			c.remarks, c.expire, c.timestamp)
	}

	swept := sweepIDs(t, models.TableSellOrders, now)
	sweptB := sweepIDsWithoutExpireDate(t, models.TableSellOrders, "BTC_JPY", now)
	rolled := rolloverIDs(t, now)

	for _, c := range cases {
		if !swept[c.orderID] && !sweptB[c.orderID] && !rolled[c.orderID] {
			t.Errorf("%s がどのジョブの対象にもならず恒久的に滞留する", c.orderID)
		}
	}
}
