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

/*
G2 のテスト。

売り注文の部分約定はシステムが損益計算に対応していない（F27はスコープ外と決定）。
発生時は手作業で記録を補うしかなく、その手作業には平均約定価格(average_price)が要る。
ところが**キャンセルした注文は取引所APIから即座に消える**ため、
この通知が average_price の唯一の記録になる。
値が1つでも欠けると手動対応が不可能になるので、通知の中身を固定しておく。
*/

// 手動対応の案内に必要な実値がすべて含まれていること（パターン①・②共通の本文）。
func TestPartialFillManualFixNoteContainsAllValues(t *testing.T) {
	snapshot := rolloverOrderSnapshot{
		found: true, state: "ACTIVE", size: 0.03, executedSize: 0.01, outstandingSize: 0.02,
		averagePrice: 511000,
	}
	record := partialFillTestRecord()
	record.Size = 0.02
	note := partialFillManualFixNote(record, snapshot, 0.03, 0.02)

	wants := []string{
		"対応していません",            // システムが未対応であること
		"過小に出ます",              // 損益レポートが実態より過小になること
		"手動での記録補正が必要",         // 手動対応が必要であること
		"BUY-PARENT",          // 親買い注文ID
		"OLD-SELL",            // 売り注文ID
		"ETH_JPY",             // product_code
		"511000",              // 平均約定価格
		"0.01",                // 約定数量
		"0.03",                // 元size
		"0.02",                // 残数量
		partialFillDocSection, // CLAUDE.md のセクション名
		"CLAUDE.md",
	}
	for _, want := range wants {
		if !strings.Contains(note, want) {
			t.Errorf("通知に %q が含まれていない: %s", want, note)
		}
	}
}

// パターン①（残数量が最小取引単位以上でローリング続行）の通知にも手動対応の案内が載ること。
func TestPartialFillRolloverMessageAsksForManualFix(t *testing.T) {
	snapshot := rolloverOrderSnapshot{
		found: true, state: "ACTIVE", size: 0.03, executedSize: 0.01, outstandingSize: 0.02,
		averagePrice: 511000,
	}
	record := partialFillTestRecord()
	record.Size = 0.02 // adjustSizeForPartialFill が残数量へ補正した後の状態
	msg := partialFillRolloverMessage(record, snapshot, 0.03, 0.02)

	// 従来からの本文（リグレッション）
	for _, want := range []string{"部分約定を検出", "残数量で再発注します", "OLD-SELL", "ETH_JPY"} {
		if !strings.Contains(msg, want) {
			t.Errorf("通知に %q が含まれていない: %s", want, msg)
		}
	}
	// G2 で追加した本文
	for _, want := range []string{"平均約定価格=511000", "手動での記録補正が必要", partialFillDocSection} {
		if !strings.Contains(msg, want) {
			t.Errorf("通知に %q が含まれていない: %s", want, msg)
		}
	}
}

// パターン②（残数量が最小取引単位未満で中止）の通知にも平均約定価格と CLAUDE.md 参照が載ること。
func TestPartialFillSkipMessageAsksForManualFix(t *testing.T) {
	for _, state := range []string{"ACTIVE", "CANCELED"} {
		snapshot := rolloverOrderSnapshot{
			found: true, state: state, size: 0.015, executedSize: 0.008, outstandingSize: 0.007,
			averagePrice: 511000,
		}
		msg := partialFillSkipMessage(partialFillTestRecord(), snapshot, 0.01, 0.015, 0.007)

		for _, want := range []string{"平均約定価格=511000", "手動での記録補正が必要", partialFillDocSection, "CLAUDE.md"} {
			if !strings.Contains(msg, want) {
				t.Errorf("state=%q: 通知に %q が含まれていない: %s", state, want, msg)
			}
		}
		// 「注文をそのまま残す → 期限切れ → 裸の保有」という経過が伝わること
		if !strings.Contains(msg, "裸の保有") {
			t.Errorf("state=%q: 裸の保有になる旨が伝わらない: %s", state, msg)
		}
	}
}

// ACTIVE のまま残す場合、いずれ期限切れで裸の保有になることを伝えること。
func TestPartialFillSkipMessageActiveWarnsAboutExpiry(t *testing.T) {
	snapshot := rolloverOrderSnapshot{
		found: true, state: "ACTIVE", size: 0.015, executedSize: 0.008, outstandingSize: 0.007,
		averagePrice: 511000,
	}
	msg := partialFillSkipMessage(partialFillTestRecord(), snapshot, 0.01, 0.015, 0.007)

	if !strings.Contains(msg, "失効") {
		t.Errorf("期限切れで失効する旨が含まれていない: %s", msg)
	}
}
