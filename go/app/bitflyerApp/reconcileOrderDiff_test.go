package bitflyerApp

import (
	"strings"
	"testing"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

/*
F7 の回帰テスト。

sell_orders は取引所から同期する仕組みが無いため、ユーザーが手動保有ポジションを
売却するために取引所へ置いた SELL 指値は約定するまで毎日「取引所のみ ACTIVE」として
エラー通知され続けてしまう。手動保有はシステムから存在しないものとして扱う方針に合わせ、
SELL 側の「取引所のみ ACTIVE」はサマリ掲載のみに落とす。

DB・API・Slack に触れない純粋関数（classifyOrderDiffs）を検証する。
*/

func buyDiff(dbOnly, exchangeOnly []string) orderDiff {
	return orderDiff{
		ProductCode: "BTC_JPY", Side: "BUY", Table: models.TableBuyOrders,
		DBOnly: dbOnly, ExchangeOnly: exchangeOnly,
	}
}

func sellDiff(dbOnly, exchangeOnly []string) orderDiff {
	return orderDiff{
		ProductCode: "ETH_JPY", Side: "SELL", Table: models.TableSellOrders,
		DBOnly: dbOnly, ExchangeOnly: exchangeOnly,
	}
}

// 手動売却の SELL 指値はエラー通知せず、サマリにのみ載ること。
func TestClassifyOrderDiffsSellExchangeOnlyIsInfoOnly(t *testing.T) {
	details := classifyOrderDiffs([]orderDiff{sellDiff(nil, []string{"MANUAL-SELL-1", "MANUAL-SELL-2"})})

	if details.AlertCount != 0 {
		t.Errorf("SELL の取引所のみ ACTIVE はアラート対象外の想定: AlertCount=%d alerts=%v", details.AlertCount, details.Alerts)
	}
	if len(details.Alerts) != 0 {
		t.Errorf("アラート明細は空の想定: %v", details.Alerts)
	}
	if details.InfoCount != 2 {
		t.Errorf("InfoCount: got %d, want 2", details.InfoCount)
	}
	// 通知を消してしまうと気づけなくなるため、サマリには必ず order_id を載せる
	joined := strings.Join(details.Info, "\n")
	for _, want := range []string{"MANUAL-SELL-1", "MANUAL-SELL-2", "ETH_JPY", "アラート対象外"} {
		if !strings.Contains(joined, want) {
			t.Errorf("サマリ明細に %q が含まれていない: %s", want, joined)
		}
	}
}

// BUY 側の「取引所のみ ACTIVE」は従来どおりアラート対象であること（回帰確認）。
func TestClassifyOrderDiffsBuyExchangeOnlyStaysAlert(t *testing.T) {
	details := classifyOrderDiffs([]orderDiff{buyDiff(nil, []string{"ORPHAN-BUY"})})

	if details.AlertCount != 1 {
		t.Fatalf("BUY の取引所のみ ACTIVE はアラート対象の想定: AlertCount=%d", details.AlertCount)
	}
	if !strings.Contains(strings.Join(details.Alerts, "\n"), "ORPHAN-BUY") {
		t.Errorf("アラート明細に order_id が含まれていない: %v", details.Alerts)
	}
	if details.InfoCount != 0 {
		t.Errorf("InfoCount: got %d, want 0", details.InfoCount)
	}
}

// 「DBのみ UNFILLED」は buy / sell どちらもアラート対象のままであること（回帰確認）。
func TestClassifyOrderDiffsDBOnlyStaysAlertForBothSides(t *testing.T) {
	details := classifyOrderDiffs([]orderDiff{
		buyDiff([]string{"GHOST-BUY"}, nil),
		sellDiff([]string{"GHOST-SELL"}, nil),
	})

	if details.AlertCount != 2 {
		t.Fatalf("DBのみ UNFILLED は売買方向を問わずアラート対象の想定: AlertCount=%d", details.AlertCount)
	}
	joined := strings.Join(details.Alerts, "\n")
	for _, want := range []string{"GHOST-BUY", "GHOST-SELL"} {
		if !strings.Contains(joined, want) {
			t.Errorf("アラート明細に %q が含まれていない: %s", want, joined)
		}
	}
	if details.InfoCount != 0 {
		t.Errorf("InfoCount: got %d, want 0", details.InfoCount)
	}
}

// アラート対象とサマリ掲載が混在する場合、それぞれ正しく振り分けられること。
func TestClassifyOrderDiffsMixed(t *testing.T) {
	details := classifyOrderDiffs([]orderDiff{
		buyDiff([]string{"GHOST-BUY"}, []string{"ORPHAN-BUY"}),
		sellDiff(nil, []string{"MANUAL-SELL"}),
	})

	if details.AlertCount != 2 || details.InfoCount != 1 {
		t.Fatalf("AlertCount/InfoCount: got %d/%d, want 2/1", details.AlertCount, details.InfoCount)
	}
	if strings.Contains(strings.Join(details.Alerts, "\n"), "MANUAL-SELL") {
		t.Errorf("手動売却の SELL がアラート明細に混ざっている: %v", details.Alerts)
	}
	if strings.Contains(strings.Join(details.Info, "\n"), "GHOST-BUY") {
		t.Errorf("アラート対象がサマリ明細に混ざっている: %v", details.Info)
	}
}

// 乖離が無ければ明細も件数も空であること。
func TestClassifyOrderDiffsNoDiff(t *testing.T) {
	details := classifyOrderDiffs([]orderDiff{buyDiff(nil, nil), sellDiff(nil, nil)})
	if details.AlertCount != 0 || details.InfoCount != 0 ||
		len(details.Alerts) != 0 || len(details.Info) != 0 {
		t.Errorf("乖離なしでは空の想定: %+v", details)
	}
}

/*
F16 の回帰テスト。

[ROLLOVER_PENDING] 付きレコードは「キャンセル成功・再発注失敗」の状態で、
取引所側に注文が無いのは当然なので必ず「DBのみ UNFILLED」として現れる。
reconcileRolloverPending の専用通知と合わせて同じ事象で毎日2通のエラーが飛んでいたため、
注文突合側では DBOnly から外してサマリ掲載に落とす。
*/

// [ROLLOVER_PENDING] 付きは DBOnly ではなく RolloverPending に振り分けられること。
func TestBuildOrderDiffSeparatesRolloverPending(t *testing.T) {
	dbIDs := []string{"S-PENDING", "S-GHOST", "S-ACTIVE"}
	exchangeIDs := map[string]bool{"S-ACTIVE": true, "S-ORPHAN": true}
	pendingIDs := map[string]bool{"S-PENDING": true}

	diff := buildOrderDiff("ETH_JPY", "SELL", models.TableSellOrders, dbIDs, exchangeIDs, pendingIDs)

	if len(diff.DBOnly) != 1 || diff.DBOnly[0] != "S-GHOST" {
		t.Errorf("DBOnly: got %v, want [S-GHOST]", diff.DBOnly)
	}
	if len(diff.RolloverPending) != 1 || diff.RolloverPending[0] != "S-PENDING" {
		t.Errorf("RolloverPending: got %v, want [S-PENDING]", diff.RolloverPending)
	}
	if len(diff.ExchangeOnly) != 1 || diff.ExchangeOnly[0] != "S-ORPHAN" {
		t.Errorf("ExchangeOnly: got %v, want [S-ORPHAN]", diff.ExchangeOnly)
	}
}

// [ROLLOVER_PENDING] が無い場合は従来どおりの突合結果になること（リグレッション）。
func TestBuildOrderDiffWithoutRolloverPending(t *testing.T) {
	dbIDs := []string{"B-1", "B-2"}
	exchangeIDs := map[string]bool{"B-2": true, "B-3": true}

	diff := buildOrderDiff("BTC_JPY", "BUY", models.TableBuyOrders, dbIDs, exchangeIDs, map[string]bool{})

	if len(diff.DBOnly) != 1 || diff.DBOnly[0] != "B-1" {
		t.Errorf("DBOnly: got %v, want [B-1]", diff.DBOnly)
	}
	if len(diff.ExchangeOnly) != 1 || diff.ExchangeOnly[0] != "B-3" {
		t.Errorf("ExchangeOnly: got %v, want [B-3]", diff.ExchangeOnly)
	}
	if len(diff.RolloverPending) != 0 {
		t.Errorf("RolloverPending: got %v, want []", diff.RolloverPending)
	}
}

// 取引所にも存在する [ROLLOVER_PENDING] 付きレコードは乖離として扱わないこと。
func TestBuildOrderDiffRolloverPendingStillActiveIsNotDiff(t *testing.T) {
	diff := buildOrderDiff("ETH_JPY", "SELL", models.TableSellOrders,
		[]string{"S-PENDING"}, map[string]bool{"S-PENDING": true}, map[string]bool{"S-PENDING": true})

	if len(diff.DBOnly) != 0 || len(diff.RolloverPending) != 0 || len(diff.ExchangeOnly) != 0 {
		t.Errorf("乖離なしの想定: %+v", diff)
	}
}

// [ROLLOVER_PENDING] はエラー通知ではなくサマリ掲載に一本化されること。
func TestClassifyOrderDiffsRolloverPendingIsInfoOnly(t *testing.T) {
	diff := sellDiff([]string{"S-GHOST"}, nil)
	diff.RolloverPending = []string{"S-PENDING"}
	details := classifyOrderDiffs([]orderDiff{diff})

	if details.AlertCount != 1 {
		t.Fatalf("AlertCount: got %d, want 1（DBのみ UNFILLED の1件だけ）", details.AlertCount)
	}
	if strings.Contains(strings.Join(details.Alerts, "\n"), "S-PENDING") {
		t.Errorf("[ROLLOVER_PENDING] がエラー通知に混ざっている: %v", details.Alerts)
	}
	if details.InfoCount != 1 {
		t.Fatalf("InfoCount: got %d, want 1", details.InfoCount)
	}
	joined := strings.Join(details.Info, "\n")
	for _, want := range []string{"S-PENDING", models.RemarkRolloverPending, "専用通知に一本化"} {
		if !strings.Contains(joined, want) {
			t.Errorf("サマリ明細に %q が含まれていない: %s", want, joined)
		}
	}
}

// order_id 集合の生成（空文字は取り込まない）。
func TestRolloverPendingOrderIDs(t *testing.T) {
	ids := rolloverPendingOrderIDs([]models.OrderRecord{
		{OrderID: "S-1"}, {OrderID: ""}, {OrderID: "S-2"},
	})
	if len(ids) != 2 || !ids["S-1"] || !ids["S-2"] {
		t.Errorf("order_id 集合: got %v, want {S-1, S-2}", ids)
	}
}

/*
E1 の回帰テスト。

日次サマリの「取引所のみ:N件(うちアラート対象外 SELL:M件)」は "取引所のみ" の内訳だが、
F7（ExchangeOnly/SELL をサマリ掲載へ）と F16（[ROLLOVER_PENDING] を専用通知へ一本化）が
重なった結果、M に InfoCount をそのまま使っており [ROLLOVER_PENDING]（＝DBのみ UNFILLED 側）
の件数が混ざっていた。件数が "取引所のみ" を上回ることさえある。
明細行のラベル自体は正しいので、集計だけを分離する。
*/

// 「取引所のみ SELL」の件数に [ROLLOVER_PENDING] を混ぜないこと。
func TestClassifyOrderDiffsSeparatesExchangeOnlySellCount(t *testing.T) {
	sell := sellDiff(nil, []string{"MANUAL-SELL-1", "MANUAL-SELL-2"})
	sell.RolloverPending = []string{"S-PENDING-1", "S-PENDING-2", "S-PENDING-3"}

	details := classifyOrderDiffs([]orderDiff{sell})

	if details.ExchangeOnlySellCount != 2 {
		t.Errorf("ExchangeOnlySellCount = %d, want 2（取引所のみ SELL の件数）", details.ExchangeOnlySellCount)
	}
	if details.RolloverPendingCount != 3 {
		t.Errorf("RolloverPendingCount = %d, want 3", details.RolloverPendingCount)
	}
	// InfoCount は「サマリ掲載のみの明細」の総数なので従来どおり合算のまま
	if details.InfoCount != 5 {
		t.Errorf("InfoCount = %d, want 5", details.InfoCount)
	}
	if details.AlertCount != 0 {
		t.Errorf("AlertCount = %d, want 0", details.AlertCount)
	}
}

// [ROLLOVER_PENDING] だけがある場合、取引所のみ SELL の件数は0であること。
func TestClassifyOrderDiffsRolloverPendingOnlyHasNoExchangeOnlySell(t *testing.T) {
	sell := sellDiff(nil, nil)
	sell.RolloverPending = []string{"S-PENDING-1"}

	details := classifyOrderDiffs([]orderDiff{sell})

	if details.ExchangeOnlySellCount != 0 {
		t.Errorf("ExchangeOnlySellCount = %d, want 0（取引所のみ SELL は0件）", details.ExchangeOnlySellCount)
	}
	if details.RolloverPendingCount != 1 || details.InfoCount != 1 {
		t.Errorf("RolloverPendingCount/InfoCount = %d/%d, want 1/1",
			details.RolloverPendingCount, details.InfoCount)
	}
}

// リグレッション: BUY の「取引所のみ」はアラート対象のままで、SELL の件数に混ざらないこと。
func TestClassifyOrderDiffsBuyExchangeOnlyIsNotCountedAsSell(t *testing.T) {
	details := classifyOrderDiffs([]orderDiff{
		buyDiff(nil, []string{"ORPHAN-BUY-1", "ORPHAN-BUY-2"}),
		sellDiff(nil, []string{"MANUAL-SELL"}),
	})

	if details.AlertCount != 2 {
		t.Errorf("AlertCount = %d, want 2", details.AlertCount)
	}
	if details.ExchangeOnlySellCount != 1 {
		t.Errorf("ExchangeOnlySellCount = %d, want 1（BUY を含めない）", details.ExchangeOnlySellCount)
	}
	if details.RolloverPendingCount != 0 {
		t.Errorf("RolloverPendingCount = %d, want 0", details.RolloverPendingCount)
	}
}

/*
「取引所のみ:N件」と「うちアラート対象外 SELL:M件」の整合を確認する。

M は N の内訳なので M <= N でなければならない。修正前は InfoCount を使っており、
[ROLLOVER_PENDING] があると M > N になりえた。
*/
func TestExchangeOnlySellCountNeverExceedsExchangeOnlyCount(t *testing.T) {
	sell := sellDiff(nil, []string{"MANUAL-SELL"})
	sell.RolloverPending = []string{"S-PENDING-1", "S-PENDING-2", "S-PENDING-3"}
	summary := &reconcileSummary{orderDiffs: []orderDiff{sell}}
	summary.diffDetails = classifyOrderDiffs(summary.orderDiffs)

	if summary.diffDetails.ExchangeOnlySellCount > summary.exchangeOnlyCount() {
		t.Errorf("うちアラート対象外 SELL:%d件 が 取引所のみ:%d件 を上回っている",
			summary.diffDetails.ExchangeOnlySellCount, summary.exchangeOnlyCount())
	}
	if summary.diffDetails.ExchangeOnlySellCount != summary.exchangeOnlyCount() {
		t.Errorf("この構成では全件が SELL のためすべてアラート対象外の想定: %d / %d",
			summary.diffDetails.ExchangeOnlySellCount, summary.exchangeOnlyCount())
	}
}
