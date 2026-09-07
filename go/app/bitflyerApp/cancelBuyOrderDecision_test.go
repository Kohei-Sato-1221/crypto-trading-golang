package bitflyerApp

import (
	"testing"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

/*
F10 の回帰テスト。

order.Timestamp.After(threshold) は Timestamp がゼロ値（DB値のパース失敗 / NULL）のとき
常に false になり、「十分古い」と解釈されてキャンセルへ進んでしまう。
scanOrderRecords の toUTCTime が変換失敗を黙ってゼロ値のまま通していたため、
パース失敗が実在する買い注文のキャンセルに直結していた。
方式B(sweep)が同じケースを判定保留にしているのと非対称でもあった。

判定を decideCancelBuyOrder()（API・DB・Slackに触れない純粋関数）に切り出したので、
ここでフェイルセーフの方向を検証する。
*/

func cancelTestOrder(orderID string, timestamp time.Time, timestampValid bool, expireDate *time.Time) models.OrderRecord {
	return models.OrderRecord{
		Table:          models.TableBuyOrders,
		OrderID:        orderID,
		ProductCode:    "BTC_JPY",
		Side:           "BUY",
		Price:          5000000,
		Size:           0.001,
		Status:         "UNFILLED",
		Timestamp:      timestamp,
		TimestampValid: timestampValid,
		ExpireDate:     expireDate,
	}
}

func TestDecideCancelBuyOrder(t *testing.T) {
	now := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	threshold := now.AddDate(0, 0, -7)

	futureExpire := now.AddDate(0, 0, 3)
	pastExpire := now.AddDate(0, 0, -1)

	tests := []struct {
		name  string
		order models.OrderRecord
		want  cancelBuyOrderDecision
	}{
		{
			name:  "期限内かつ7日を超えて未約定ならキャンセル（回帰: 従来どおり）",
			order: cancelTestOrder("ID-OLD", now.AddDate(0, 0, -10), true, &futureExpire),
			want:  cancelBuyOrderCancel,
		},
		{
			name:  "expire_date が NULL でも7日超ならキャンセル（回帰: 旧レコードの救済）",
			order: cancelTestOrder("ID-OLD-NOEXP", now.AddDate(0, 0, -10), true, nil),
			want:  cancelBuyOrderCancel,
		},
		{
			name:  "7日に満たなければ残す（回帰）",
			order: cancelTestOrder("ID-NEW", now.AddDate(0, 0, -3), true, &futureExpire),
			want:  cancelBuyOrderKeep,
		},
		{
			name:  "閾値ちょうどはキャンセル（After は厳密比較）",
			order: cancelTestOrder("ID-BOUNDARY", threshold, true, &futureExpire),
			want:  cancelBuyOrderCancel,
		},
		{
			name:  "既に期限切れならキャンセルAPIを叩かない（回帰: expireSweepJobの担当）",
			order: cancelTestOrder("ID-EXPIRED", now.AddDate(0, 0, -10), true, &pastExpire),
			want:  cancelBuyOrderSkipExpired,
		},
		{
			name:  "timestamp がゼロ値ならキャンセルしない（F10 の本丸）",
			order: cancelTestOrder("ID-ZEROTS", time.Time{}, false, &futureExpire),
			want:  cancelBuyOrderSkipInvalidTimestamp,
		},
		{
			name:  "TimestampValid=false ならゼロ値でなくてもキャンセルしない",
			order: cancelTestOrder("ID-INVALID", now.AddDate(0, 0, -10), false, &futureExpire),
			want:  cancelBuyOrderSkipInvalidTimestamp,
		},
		{
			name:  "timestamp がゼロ値でも期限切れ判定が先（expireSweepJobの担当のまま）",
			order: cancelTestOrder("ID-ZEROTS-EXPIRED", time.Time{}, false, &pastExpire),
			want:  cancelBuyOrderSkipExpired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decideCancelBuyOrder(tt.order, now, threshold); got != tt.want {
				t.Errorf("decideCancelBuyOrder() = %v, want %v", got, tt.want)
			}
		})
	}
}

/*
旧実装（TimestampValid を見ずに Timestamp.After(threshold) だけで判定）だと
ゼロ値がキャンセル側に倒れることを、同じ入力で確認しておく。
この差分が F10 の修正内容そのものになる。
*/
func TestZeroTimestampWouldFallToCancelWithoutGuard(t *testing.T) {
	now := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	threshold := now.AddDate(0, 0, -7)
	zero := time.Time{}

	if zero.After(threshold) {
		t.Fatal("precondition: ゼロ値は threshold より後にならない")
	}
	// 旧実装は !After(threshold) をもって「十分古い」と解釈しキャンセルへ進んでいた
	futureExpire := now.AddDate(0, 0, 3)
	order := cancelTestOrder("ID-ZEROTS", zero, false, &futureExpire)
	if got := decideCancelBuyOrder(order, now, threshold); got == cancelBuyOrderCancel {
		t.Error("timestamp が解釈できないレコードをキャンセルしてはならない")
	}
}
