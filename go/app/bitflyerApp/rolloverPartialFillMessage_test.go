package bitflyerApp

import (
	"strings"
	"testing"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

/*
F13 の回帰テスト。

「部分約定で残数量が最小取引単位未満」の通知文が「注文はそのまま残します」で固定されていた。
この通知は取引所側で既に CANCELED になった注文（default 分岐）からも出るため、
実際には売り注文がもう存在せず現物だけが残る「裸の保有」なのに、
ユーザーが「まだ売り注文が生きている」と誤認して対応が遅れる。
*/

func partialFillTestRecord() models.SellOrderRecord {
	return models.SellOrderRecord{
		Table:       models.TableSellOrders,
		OrderID:     "OLD-SELL",
		ParentID:    "BUY-PARENT",
		ProductCode: "ETH_JPY",
		Side:        "SELL",
		Price:       512345,
		Size:        0.015,
	}
}

// ACTIVE のままなら「注文はそのまま残す」が実態であること（リグレッション）。
func TestPartialFillSkipMessageActiveKeepsOrder(t *testing.T) {
	snapshot := rolloverOrderSnapshot{
		found: true, state: "ACTIVE", size: 0.015, executedSize: 0.008, outstandingSize: 0.007,
	}
	msg := partialFillSkipMessage(partialFillTestRecord(), snapshot, 0.01, 0.015, 0.007)

	if !strings.Contains(msg, "注文はそのまま残します") {
		t.Errorf("ACTIVE では注文が残る旨を伝えること: %s", msg)
	}
	if strings.Contains(msg, "裸の保有」の状態") {
		t.Errorf("ACTIVE なのに裸の保有と通知している: %s", msg)
	}
	// 開発ルール: 通知には OrderID / Price / Size のコンテキストを含めること
	for _, want := range []string{"OLD-SELL", "BUY-PARENT", "ETH_JPY", "512345"} {
		if !strings.Contains(msg, want) {
			t.Errorf("通知に %q が含まれていない: %s", want, msg)
		}
	}
}

// 取引所側で既にキャンセル済みなら「裸の保有」であることを伝えること。
func TestPartialFillSkipMessageCancelledSaysNakedHolding(t *testing.T) {
	snapshot := rolloverOrderSnapshot{
		found: true, state: "CANCELED", size: 0.015, executedSize: 0.008, outstandingSize: 0,
	}
	msg := partialFillSkipMessage(partialFillTestRecord(), snapshot, 0.01, 0.015, 0.007)

	if strings.Contains(msg, "注文はそのまま残します") {
		t.Errorf("取引所に注文が無いのに「そのまま残す」と通知している: %s", msg)
	}
	for _, want := range []string{"裸の保有", "CANCELED", "再発注もできません"} {
		if !strings.Contains(msg, want) {
			t.Errorf("通知に %q が含まれていない: %s", want, msg)
		}
	}
	// 裸の保有は最優先で気づく必要があるため 🚨🚨 で通知する
	if !strings.HasPrefix(msg, "🚨🚨") {
		t.Errorf("裸の保有は 🚨🚨 で通知すること: %s", msg)
	}
}

// EXPIRED / REJECTED など ACTIVE 以外も同じ扱いになること。
func TestPartialFillSkipMessageNonActiveStates(t *testing.T) {
	for _, state := range []string{"EXPIRED", "REJECTED", ""} {
		snapshot := rolloverOrderSnapshot{
			found: true, state: state, size: 0.015, executedSize: 0.008, outstandingSize: 0,
		}
		msg := partialFillSkipMessage(partialFillTestRecord(), snapshot, 0.01, 0.015, 0.007)
		if !strings.Contains(msg, "裸の保有") {
			t.Errorf("state=%q でも裸の保有として通知すること: %s", state, msg)
		}
	}
}
