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
