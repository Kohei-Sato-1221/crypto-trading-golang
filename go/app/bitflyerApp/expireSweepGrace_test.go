package bitflyerApp

import (
	"testing"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/config"
)

/*
E3 の回帰テスト。

expireSweepJob が「期限切れ」とみなす判定境界は「sweep 実行時刻 − 猶予」である。
猶予が 10分 だと境界が 06:05 - 10分 = 05:55 となり、rolloverSellOrderJob
（05:30 開始・対象件数によって最大25分程度かかりうる）の実行時間帯と重なる。
この窓では、ローリングがキャンセル→再発注している最中のレコードを sweep が
同じ日のうちに拾いうる（decideSweepAction のおかげでDBは壊れないが、
🚨🚨の失効通知が誤って飛ぶ）。

猶予を 45分 にすると境界が 05:20 となりローリング開始より前になり、窓が消える。
スケジュールと猶予はどちらも設定値なので、片方だけ変更されて窓が復活しないよう
不変条件そのものをテストで固定する。
*/

// parseTriggerTime は "HH:MM" 形式のジョブ実行時刻を、その日の 00:00 からの経過時間へ変換する。
func parseTriggerTime(t *testing.T, value string) time.Duration {
	t.Helper()

	parsed, err := time.Parse("15:04", value)
	if err != nil {
		t.Fatalf("trigger time %q をパースできない: %v", value, err)
	}
	return time.Duration(parsed.Hour())*time.Hour + time.Duration(parsed.Minute())*time.Minute
}

// sweep の判定境界がローリング開始時刻より前であること（同日競合ウィンドウが無いこと）。
func TestExpireSweepGraceHasNoOverlapWithRollover(t *testing.T) {
	rolloverStart := parseTriggerTime(t, config.DefaultTriggerTime05) // 05:30
	sweepStart := parseTriggerTime(t, config.DefaultTriggerTime06)    // 06:05
	grace := time.Duration(config.DefaultExpireSweepGraceMinutes) * time.Minute

	boundary := sweepStart - grace
	if boundary > rolloverStart {
		t.Errorf("sweep の判定境界 %v がローリング開始 %v より後。"+
			"ローリング中のレコードを同じ日の sweep が拾い、誤った失効通知が飛ぶ "+
			"(sweep=%s grace=%d分)",
			boundary, rolloverStart, config.DefaultTriggerTime06, config.DefaultExpireSweepGraceMinutes)
	}

	// sweep は買い注文ジョブ(06:30)より前に終わってスロットを解放する必要がある
	if sweepStart >= parseTriggerTime(t, config.DefaultTriggerTime01) {
		t.Errorf("sweep(%s) が買い注文(%s)より後になっている",
			config.DefaultTriggerTime06, config.DefaultTriggerTime01)
	}
}

// 猶予の既定値が想定どおりであること（意図しない巻き戻しの検知）。
func TestExpireSweepGraceDefault(t *testing.T) {
	if config.DefaultExpireSweepGraceMinutes != 45 {
		t.Errorf("DefaultExpireSweepGraceMinutes = %d, want 45（E3 で 10 から変更）",
			config.DefaultExpireSweepGraceMinutes)
	}
}

/*
リグレッション: expireSweepGrace() の既定値フォールバックが従来どおり働くこと。

config.Config は未初期化（BFExpireSweepGraceMinutes = 0）なので、
0以下のときに既定値へフォールバックする分岐が働く。
*/
func TestExpireSweepGraceFallsBackToDefault(t *testing.T) {
	original := config.Config.BFExpireSweepGraceMinutes
	defer func() { config.Config.BFExpireSweepGraceMinutes = original }()

	for _, invalid := range []int{0, -1} {
		config.Config.BFExpireSweepGraceMinutes = invalid
		want := time.Duration(config.DefaultExpireSweepGraceMinutes) * time.Minute
		if got := expireSweepGrace(); got != want {
			t.Errorf("expireSweepGrace() with %d = %v, want %v", invalid, got, want)
		}
	}

	// 設定値がある場合はそれを使うこと
	config.Config.BFExpireSweepGraceMinutes = 20
	if got := expireSweepGrace(); got != 20*time.Minute {
		t.Errorf("expireSweepGrace() = %v, want 20m", got)
	}
}
