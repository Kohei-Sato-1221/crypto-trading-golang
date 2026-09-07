package bitflyerApp

import (
	"testing"
	"time"
)

/*
F18 の回帰テスト。

売り注文の上限超過は解消まで数週間続きうるのに、買い注文ジョブは1日に最大12本走るため
同一内容の 🚨 が1日に何通も流れ、本物の通知が埋もれていた。
通知を消すのではなく間隔をあけて鳴らすため、notificationThrottle で集約する。

時刻に依存する処理だが now を引数に取る純粋な作りなので、実時間を待たずに検証できる。
*/

// 初回は必ず通知し、間隔内は抑制され、間隔を過ぎれば再び通知すること。
func TestNotificationThrottleSuppressesWithinInterval(t *testing.T) {
	throttle := newNotificationThrottle(24 * time.Hour)
	base := time.Date(2026, 9, 7, 9, 0, 0, 0, time.UTC)

	if !throttle.shouldNotify(sellOverLimitThrottleKey, base) {
		t.Fatal("初回は通知する想定")
	}
	// 同日中の後続ジョブ（1日最大12本）はすべて抑制される
	for _, after := range []time.Duration{time.Second, 3 * time.Hour, 23*time.Hour + 59*time.Minute} {
		if throttle.shouldNotify(sellOverLimitThrottleKey, base.Add(after)) {
			t.Errorf("間隔内(%v後)は抑制する想定", after)
		}
	}
	// 翌日は再び通知する（気づけなくならないよう通知は消さない）
	if !throttle.shouldNotify(sellOverLimitThrottleKey, base.Add(24*time.Hour)) {
		t.Error("間隔を過ぎたら再び通知する想定")
	}
	// 再通知後はそこを起点に再び抑制される
	if throttle.shouldNotify(sellOverLimitThrottleKey, base.Add(25*time.Hour)) {
		t.Error("再通知の直後は抑制する想定")
	}
}

// 事象（key）ごとに独立して抑制されること。
func TestNotificationThrottleIsPerKey(t *testing.T) {
	throttle := newNotificationThrottle(24 * time.Hour)
	now := time.Date(2026, 9, 7, 9, 0, 0, 0, time.UTC)

	if !throttle.shouldNotify("a", now) || !throttle.shouldNotify("b", now) {
		t.Fatal("別々の key は互いに影響しない想定")
	}
	if throttle.shouldNotify("a", now.Add(time.Hour)) {
		t.Error("key a は抑制される想定")
	}
	if throttle.shouldNotify("b", now.Add(time.Hour)) {
		t.Error("key b は抑制される想定")
	}
}

/*
システム時刻が巻き戻った場合は抑制しないこと。

Raspberry Pi には RTC が無く、起動直後の時刻は不正確で NTP 同期時に巻き戻ることがある。
時計の異常で通知が止まる方が危険なため、判定不能なら鳴らす側に倒す。
*/
func TestNotificationThrottleNotifiesWhenClockGoesBackwards(t *testing.T) {
	throttle := newNotificationThrottle(24 * time.Hour)
	now := time.Date(2026, 9, 7, 9, 0, 0, 0, time.UTC)

	if !throttle.shouldNotify(sellOverLimitThrottleKey, now) {
		t.Fatal("初回は通知する想定")
	}
	if !throttle.shouldNotify(sellOverLimitThrottleKey, now.Add(-2*time.Hour)) {
		t.Error("時刻が巻き戻った場合は抑制せず通知する想定")
	}
}

// interval が 0 なら常に通知すること（抑制を無効化できる）。
func TestNotificationThrottleZeroIntervalAlwaysNotifies(t *testing.T) {
	throttle := newNotificationThrottle(0)
	now := time.Date(2026, 9, 7, 9, 0, 0, 0, time.UTC)

	if !throttle.shouldNotify("k", now) || !throttle.shouldNotify("k", now.Add(time.Nanosecond)) {
		t.Error("interval=0 では常に通知する想定")
	}
}

// 集約間隔は24時間（1日1通）であること。
func TestSellOverLimitNotifyInterval(t *testing.T) {
	if sellOverLimitNotifyInterval != 24*time.Hour {
		t.Errorf("sellOverLimitNotifyInterval: got %v, want 24h", sellOverLimitNotifyInterval)
	}
}
