package bitflyerApp

import (
	"sync"
	"time"
)

/*
notificationThrottle は同一事象の繰り返しSlack通知を一定間隔まで抑制する。

売り注文の上限超過のように「解消まで数週間続きうる状態」は、ジョブが走るたびに
同じ内容の 🚨 が飛ぶ。買い注文ジョブは1日に最大12本（BTC/ETH × 曜日戦略）走るため、
超過状態が続くと同一内容の警告が1日に何通も流れ、本物の通知が埋もれてしまう。

通知そのものを消すと気づけなくなるため、消すのではなく間隔をあけて鳴らす。
状態はプロセス内のメモリにのみ持ち、再起動すると必ず1回鳴る（鳴らしすぎるより
鳴らさない方が危険なため、迷ったら鳴らす側に倒す）。
*/
type notificationThrottle struct {
	mu       sync.Mutex
	interval time.Duration
	last     map[string]time.Time
}

// newNotificationThrottle は指定間隔の抑制器を返す。
func newNotificationThrottle(interval time.Duration) *notificationThrottle {
	return &notificationThrottle{interval: interval, last: make(map[string]time.Time)}
}

/*
shouldNotify は key の事象を now 時点で通知してよいかを返す。

通知してよいと判断した場合は最終通知時刻を now に更新する（副作用がある）。
now が前回より過去（Raspberry Pi はRTCを持たず、NTP同期でシステム時刻が
巻き戻ることがある）の場合は抑制せず通知する。時計の異常で通知が止まる方が危険なため。
*/
func (t *notificationThrottle) shouldNotify(key string, now time.Time) bool {
	if t == nil {
		return true
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	last, ok := t.last[key]
	if ok && now.After(last) && now.Sub(last) < t.interval {
		return false
	}
	t.last[key] = now
	return true
}
