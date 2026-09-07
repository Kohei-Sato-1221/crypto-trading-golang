package bitflyerApp

import (
	"testing"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

/*
F8 の回帰テスト。

方式B には COMPLETED 一覧の遡り限界(oldestCompleted)との比較による判定保留があったが、
方式A（expire_date の経過）には無く、取引量が増えて遡り範囲が短くなると
「一覧に無い＝失効」と誤判定して CANCELLED に落としうる状態だった。
誤 CANCELLED になった買い注文は placeSellOrder の対象から外れ、保有した現物が
DB上追跡不能（裸の保有）になる。

判定を decideSweepAction() に共通化したので、ここでその優先順位を検証する。
（DB・API・Slack に触れない純粋関数）
*/

func sweepTestIndex(oldestCompleted time.Time, activeIDs, completedIDs []string) *exchangeOrderIndex {
	index := &exchangeOrderIndex{
		activeIDs:       map[string]bool{},
		completedIDs:    map[string]bool{},
		oldestCompleted: oldestCompleted,
	}
	for _, id := range activeIDs {
		index.activeIDs[id] = true
	}
	for _, id := range completedIDs {
		index.completedIDs[id] = true
	}
	return index
}

func sweepTestRecord(table models.OrderTable, orderID string, timestamp time.Time) models.OrderRecord {
	return models.OrderRecord{
		Table:       table,
		OrderID:     orderID,
		ProductCode: "BTC_JPY",
		Side:        "BUY",
		Price:       5000000,
		Size:        0.001,
		Status:      "UNFILLED",
		Timestamp:   timestamp,
	}
}

func TestDecideSweepAction(t *testing.T) {
	oldest := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	older := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		record    models.OrderRecord
		index     *exchangeOrderIndex
		want      sweepDecision
		wantLabel string
	}{
		{
			name:      "COMPLETED一覧にあれば約定済み扱い（CANCELLEDにしない）",
			record:    sweepTestRecord(models.TableBuyOrders, "ID-COMPLETED", newer),
			index:     sweepTestIndex(oldest, nil, []string{"ID-COMPLETED"}),
			want:      sweepDecisionFilled,
			wantLabel: "filled",
		},
		{
			name:      "ACTIVE一覧にあれば状態を変えない",
			record:    sweepTestRecord(models.TableBuyOrders, "ID-ACTIVE", newer),
			index:     sweepTestIndex(oldest, []string{"ID-ACTIVE"}, nil),
			want:      sweepDecisionSkipActive,
			wantLabel: "skipActive",
		},
		{
			name:      "COMPLETEDとACTIVEの両方にあればCOMPLETEDを優先",
			record:    sweepTestRecord(models.TableBuyOrders, "ID-BOTH", newer),
			index:     sweepTestIndex(oldest, []string{"ID-BOTH"}, []string{"ID-BOTH"}),
			want:      sweepDecisionFilled,
			wantLabel: "filled",
		},
		{
			name:      "遡り限界より新しく一覧に無ければ失効とみなす（回帰: 従来どおりCANCELLED）",
			record:    sweepTestRecord(models.TableBuyOrders, "ID-EXPIRED", newer),
			index:     sweepTestIndex(oldest, nil, nil),
			want:      sweepDecisionCancel,
			wantLabel: "cancel",
		},
		{
			name:      "遡り限界より古ければ判定保留（F8 の本丸。方式Aでも保留になる）",
			record:    sweepTestRecord(models.TableBuyOrders, "ID-TOOOLD", older),
			index:     sweepTestIndex(oldest, nil, nil),
			want:      sweepDecisionHold,
			wantLabel: "hold",
		},
		{
			name:      "遡り限界と同時刻なら保留（境界は安全側に倒す）",
			record:    sweepTestRecord(models.TableBuyOrders, "ID-BOUNDARY", oldest),
			index:     sweepTestIndex(oldest, nil, nil),
			want:      sweepDecisionHold,
			wantLabel: "hold",
		},
		{
			name:      "oldestCompletedがゼロ値なら遡り限界を確定できないので保留",
			record:    sweepTestRecord(models.TableBuyOrders, "ID-NOLIMIT", newer),
			index:     sweepTestIndex(time.Time{}, nil, nil),
			want:      sweepDecisionHold,
			wantLabel: "hold",
		},
		{
			name:      "timestampがゼロ値なら保留（DB値のパース失敗をCANCELLEDに倒さない）",
			record:    sweepTestRecord(models.TableSellOrders, "ID-NOTS", time.Time{}),
			index:     sweepTestIndex(oldest, nil, nil),
			want:      sweepDecisionHold,
			wantLabel: "hold",
		},
		{
			name:      "遡り限界より古くてもCOMPLETEDにあれば約定済み扱いが優先",
			record:    sweepTestRecord(models.TableSellOrders, "ID-OLD-COMPLETED", older),
			index:     sweepTestIndex(oldest, nil, []string{"ID-OLD-COMPLETED"}),
			want:      sweepDecisionFilled,
			wantLabel: "filled",
		},
		{
			name:      "遡り限界より古くてもACTIVEにあれば据置が優先",
			record:    sweepTestRecord(models.TableSellOrders, "ID-OLD-ACTIVE", older),
			index:     sweepTestIndex(oldest, []string{"ID-OLD-ACTIVE"}, nil),
			want:      sweepDecisionSkipActive,
			wantLabel: "skipActive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decideSweepAction(tt.record, tt.index); got != tt.want {
				t.Errorf("decideSweepAction() = %v, want %v (%s)", got, tt.want, tt.wantLabel)
			}
		})
	}
}

// 方式A・方式Bが同じ判定関数を通ることで、遡り限界のガードが両方に効くことを確認する。
func TestSweepLookbackGuardAppliesToBothMethods(t *testing.T) {
	oldest := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	older := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	index := sweepTestIndex(oldest, nil, nil)

	// 方式Aの候補（expire_date あり）と方式Bの候補（expire_date なし）で判定が一致すること
	expire := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	methodA := sweepTestRecord(models.TableBuyOrders, "ID-A", older)
	methodA.ExpireDate = &expire
	methodB := sweepTestRecord(models.TableBuyOrders, "ID-B", older)

	if got := decideSweepAction(methodA, index); got != sweepDecisionHold {
		t.Errorf("方式Aの候補が保留にならない: got=%v（遡り範囲外の約定を誤CANCELLEDにする）", got)
	}
	if got := decideSweepAction(methodB, index); got != sweepDecisionHold {
		t.Errorf("方式Bの候補が保留にならない: got=%v", got)
	}
}
