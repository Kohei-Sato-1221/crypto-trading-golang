package bitflyerApp

import (
	"strings"
	"testing"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/config"
)

/*
F5 のテスト。

placeBuyOrder は expire_date = timestamp + buy_minute_to_expire を記録し、
decideCancelBuyOrder は「expire_date <= now なら skip」を「timestamp <= now - cancelDays なら cancel」
より先に評価する。そのため cancelDays * 1440 >= buy_minute_to_expire では
キャンセル条件を満たすレコードが必ず skip に吸収され、能動キャンセルは
expire_date が NULL の旧レコードにしか効かない。

確定方針は「期限切れの後始末は expireSweepJob 任せ。出荷設定(7日/10080分)は意図してこの状態」。
設定値の取り違えでジョブが黙って無効化される事故を防ぐため、
cancelBuyOrderConfigNote() が毎回どちらのモードかを報告する。

ここでは
  1. cancelBuyOrderConfigNote() が両モードを正しく判別すること
  2. その判別が decideCancelBuyOrder() の実挙動と一致すること（構造そのものの検証）
  3. 既存の正常系（旧レコードの掃除・期限内の保持）が壊れていないこと
を検証する。
*/

func TestCancelBuyOrderConfigNote(t *testing.T) {
	tests := []struct {
		name              string
		cancelDays        int
		buyMinuteToExpire int
		wantEffective     bool
	}{
		{
			name:              "出荷設定(7日/10080分)は同値のため能動キャンセルは実質無効",
			cancelDays:        7,
			buyMinuteToExpire: 10080,
			wantEffective:     false,
		},
		{
			name:              "cancelDaysを短くすると能動キャンセルが有効になる",
			cancelDays:        5,
			buyMinuteToExpire: 10080,
			wantEffective:     true,
		},
		{
			name:              "cancelDaysが期限より長ければ当然無効",
			cancelDays:        10,
			buyMinuteToExpire: 10080,
			wantEffective:     false,
		},
		{
			name:              "cancelDays=0 は既定値7日にフォールバックし、既定期限と同値で無効",
			cancelDays:        0,
			buyMinuteToExpire: 0,
			wantEffective:     false,
		},
		{
			name:              "buy_minute_to_expire未設定でも既定値10080分で判定する",
			cancelDays:        3,
			buyMinuteToExpire: 0,
			wantEffective:     true,
		},
		{
			name:              "短期戦略相当(1440分=1日)にcancelDays=1を合わせると無効",
			cancelDays:        1,
			buyMinuteToExpire: 1440,
			wantEffective:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			note, effective := cancelBuyOrderConfigNote(tt.cancelDays, tt.buyMinuteToExpire)
			if effective != tt.wantEffective {
				t.Errorf("cancelBuyOrderConfigNote(%d, %d) effective = %v, want %v",
					tt.cancelDays, tt.buyMinuteToExpire, effective, tt.wantEffective)
			}
			if note == "" {
				t.Error("note は常に説明文を返すこと（ログに残す前提）")
			}
			// ログを読んだ人が設定値を確認できるよう、実効値を必ず含める
			if !strings.Contains(note, "buy_order_cancel_days") || !strings.Contains(note, "buy_minute_to_expire") {
				t.Errorf("note に設定キー名が含まれていない: %s", note)
			}
		})
	}
}

func TestCancelBuyOrderConfigNoteUsesDefaults(t *testing.T) {
	// 既定値そのものが同値（7日 == 10080分）であることを固定する。
	// ここが崩れると「出荷設定では sweep 任せ」という前提が黙って変わる
	if config.DefaultBuyOrderCancelDays*minutesPerDay != config.DefaultBuyMinuteToExpire {
		t.Fatalf("既定値の前提が変わっている: %d日(%d分) vs %d分",
			config.DefaultBuyOrderCancelDays,
			config.DefaultBuyOrderCancelDays*minutesPerDay,
			config.DefaultBuyMinuteToExpire)
	}
	if _, effective := cancelBuyOrderConfigNote(-1, -1); effective {
		t.Error("負値は既定値にフォールバックし、能動キャンセルは無効になるはず")
	}
}

/*
cancelBuyOrderConfigNote の判定が decideCancelBuyOrder の実挙動と一致することを、
placeBuyOrder と同じ式(expire_date = timestamp + buy_minute_to_expire)で
生成したレコードを時系列で流して確認する（F5 の本丸）。
*/
func TestCancelBuyOrderEffectivenessMatchesDecision(t *testing.T) {
	now := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name              string
		cancelDays        int
		buyMinuteToExpire int
	}{
		{name: "出荷設定 7日/10080分", cancelDays: 7, buyMinuteToExpire: 10080},
		{name: "早期解放 5日/10080分", cancelDays: 5, buyMinuteToExpire: 10080},
		{name: "長め 10日/10080分", cancelDays: 10, buyMinuteToExpire: 10080},
		{name: "短期 1日/1440分", cancelDays: 1, buyMinuteToExpire: 1440},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, effective := cancelBuyOrderConfigNote(tc.cancelDays, tc.buyMinuteToExpire)
			threshold := now.AddDate(0, 0, -tc.cancelDays)

			// 発注からの経過時間を1時間刻みで振り、
			// expire_date を持つレコードが1件でもキャンセル判定に到達するかを調べる
			anyCancel := false
			for elapsedHours := 0; elapsedHours <= 24*30; elapsedHours++ {
				timestamp := now.Add(-time.Duration(elapsedHours) * time.Hour)
				expire := timestamp.Add(time.Duration(tc.buyMinuteToExpire) * time.Minute)
				order := cancelTestOrder("ID-SIM", timestamp, true, &expire)
				if decideCancelBuyOrder(order, now, threshold) == cancelBuyOrderCancel {
					anyCancel = true
					break
				}
			}

			if anyCancel != effective {
				t.Errorf("設定点検(effective=%v)と実挙動(キャンセル到達=%v)が食い違っている", effective, anyCancel)
			}
		})
	}
}

/*
リグレッション: F5 の対応でも既存の正常系が変わっていないこと。
出荷設定でも「expire_date が NULL の旧レコード」は従来どおりキャンセルされ、
期限内で日数未達のレコードは残る。
*/
func TestCancelBuyOrderShippingConfigStillCleansLegacyRecords(t *testing.T) {
	now := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	cancelDays := config.DefaultBuyOrderCancelDays
	threshold := now.AddDate(0, 0, -cancelDays)

	legacy := cancelTestOrder("ID-LEGACY", now.AddDate(0, 0, -30), true, nil)
	if got := decideCancelBuyOrder(legacy, now, threshold); got != cancelBuyOrderCancel {
		t.Errorf("expire_date が NULL の旧レコードはキャンセル対象のままであること: got=%v", got)
	}

	fresh := now.AddDate(0, 0, -2)
	freshExpire := fresh.Add(time.Duration(config.DefaultBuyMinuteToExpire) * time.Minute)
	if got := decideCancelBuyOrder(cancelTestOrder("ID-FRESH", fresh, true, &freshExpire), now, threshold); got != cancelBuyOrderKeep {
		t.Errorf("期限内かつ日数未達のレコードは残すこと: got=%v", got)
	}

	// 期限を過ぎたレコードは従来どおり expireSweepJob に委ねる
	old := now.AddDate(0, 0, -8)
	oldExpire := old.Add(time.Duration(config.DefaultBuyMinuteToExpire) * time.Minute)
	if got := decideCancelBuyOrder(cancelTestOrder("ID-OLD", old, true, &oldExpire), now, threshold); got != cancelBuyOrderSkipExpired {
		t.Errorf("期限切れは expireSweepJob の担当のままであること: got=%v", got)
	}
}
