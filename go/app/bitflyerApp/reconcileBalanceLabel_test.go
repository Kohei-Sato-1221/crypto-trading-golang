package bitflyerApp

import (
	"strings"
	"testing"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

/*
F17 の回帰テスト。

Sprint 7 の修正で手動保有(RemarkManualHold)を AlertExpected から除外したため、
余剰の主因は「手動保有130レコード」であって「DB未追跡分」ではない。
ラベル「余剰(DB未追跡分。アラート対象外)」は untracked_holding_btc /
untracked_holding_eth の設定漏れと誤解されるため、実態に合わせて修正した。

DB・API・Slack に触れない純粋関数（balanceStateLabel）を検証する。
*/

func labelTestDiff(exchange, alertExpected, threshold, manual float64) balanceDiff {
	return balanceDiff{
		Currency:      "BTC",
		ProductCode:   "BTC_JPY",
		Exchange:      exchange,
		Holding:       models.ExpectedHolding{Manual: manual},
		AlertExpected: alertExpected,
		Diff:          exchange - alertExpected,
		Threshold:     threshold,
	}
}

// 余剰のラベルが手動保有にも言及すること（実態との食い違いの解消）。
func TestBalanceStateLabelSurplus(t *testing.T) {
	// 手動保有 0.035 BTC を判定から除外しているため、そのぶんが余剰として出る
	label := balanceStateLabel(labelTestDiff(0.045, 0.010, 0.001, 0.035))

	if !strings.Contains(label, "余剰") {
		t.Fatalf("余剰と判定される想定: got %q", label)
	}
	if !strings.Contains(label, "手動保有") {
		t.Errorf("余剰ラベルは手動保有に言及する想定: got %q", label)
	}
	if !strings.Contains(label, "アラート対象外") {
		t.Errorf("アラート対象外である旨が欠けている: got %q", label)
	}
	if strings.Contains(label, "DB未追跡分。") {
		t.Errorf("旧ラベル（DB未追跡分のみに言及）が残っている: got %q", label)
	}
}

// 不足は従来どおり 🚨 付きであること（リグレッション）。
func TestBalanceStateLabelShortfall(t *testing.T) {
	label := balanceStateLabel(labelTestDiff(0.005, 0.010, 0.001, 0))
	if label != "🚨不足" {
		t.Errorf("不足ラベル: got %q, want 🚨不足", label)
	}
}

// 閾値以内は「乖離なし」であること（リグレッション）。
func TestBalanceStateLabelWithinThreshold(t *testing.T) {
	for _, diff := range []balanceDiff{
		labelTestDiff(0.0105, 0.010, 0.001, 0), // 余剰側だが閾値以内
		labelTestDiff(0.0095, 0.010, 0.001, 0), // 不足側だが閾値以内
		labelTestDiff(0.010, 0.010, 0.001, 0),  // 完全一致
	} {
		if label := balanceStateLabel(diff); label != "乖離なし" {
			t.Errorf("閾値以内のラベル: got %q, want 乖離なし (exchange:%v expected:%v)",
				label, diff.Exchange, diff.AlertExpected)
		}
	}
}
