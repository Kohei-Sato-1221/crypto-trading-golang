package bitflyerApp

import (
	"errors"
	"strings"
	"testing"
)

/*
F30-2 のテスト。

scheduler.Run() は (*Job, error) を返すが、従来の service.go は戻り値を捨てていた。
そのため登録に失敗したジョブはゴルーチンが起動されず、ログにもSlackにも何も出ないまま
二度と発火しない（買い注文ジョブ12本が同じ trigger_time_01 を使うため、
1つの設定ミスで発注が全滅しうる）。

jobRegistry は Run() の戻り値を必ず検査して失敗を集約する。
ここでは scheduler に触れない集約ロジック（record / hasFailures / total / failureReport）
だけを検証する。実際の登録処理はゴルーチンを起動する副作用があるため実行しない。
*/

func TestJobRegistryRecordAggregatesFailures(t *testing.T) {
	r := &jobRegistry{}

	r.record("buyingBTCJobEveryDay", "06:30", nil)
	r.record("buyingETHJobEveryDay", "06:30", nil)
	r.record("sendResultsJob", "", errors.New("time format not valid"))
	r.record("deleteRecordJob", "7200秒ごと", nil)
	r.record("savePriceHistoryJob(朝)", "25:00", errors.New("time format not valid"))

	if got, want := r.registered, 3; got != want {
		t.Errorf("registered = %d, want %d", got, want)
	}
	if got, want := len(r.failures), 2; got != want {
		t.Errorf("failures = %d, want %d", got, want)
	}
	if got, want := r.total(), 5; got != want {
		t.Errorf("total() = %d, want %d", got, want)
	}
	if !r.hasFailures() {
		t.Error("hasFailures() = false, want true（1本でも失敗したら検知できなければならない）")
	}
}

/*
1本だけ失敗したケースでも取りこぼさないこと。
「20本中1本が静かに欠ける」のが今回直したい障害そのものなので、
境界として明示的に固定する。
*/
func TestJobRegistryDetectsSingleFailureAmongMany(t *testing.T) {
	r := &jobRegistry{}
	for i := 0; i < 19; i++ {
		r.record("job", "06:30", nil)
	}
	r.record("cancelBuyOrderJob", "", errors.New("time format not valid"))

	if !r.hasFailures() {
		t.Fatal("20本中1本の失敗を検知できていない")
	}
	report := r.failureReport()
	if !strings.Contains(report, "cancelBuyOrderJob") {
		t.Errorf("失敗したジョブ名が報告に含まれていない: %q", report)
	}
	if !strings.Contains(report, "1/20件") {
		t.Errorf("失敗件数と総数が報告に含まれていない: %q", report)
	}
}

/*
failureReport は「どのジョブが」「どの設定値で」「なぜ」失敗したかを
すべて列挙し、そのジョブが発火しないことを明示する。
*/
func TestJobRegistryFailureReportContainsAllFailures(t *testing.T) {
	r := &jobRegistry{}
	r.record("buyingBTCJobEveryDay", "", errors.New("time format not valid"))
	r.record("syncBTCBuyOrderJob", "90秒ごと", errors.New("cannot set recurrent time with 0"))
	r.record("expireSweepJob", "06:05", nil)

	report := r.failureReport()

	for _, want := range []string{
		"buyingBTCJobEveryDay",
		"syncBTCBuyOrderJob",
		"time format not valid",
		"cannot set recurrent time with 0",
		"90秒ごと",
		"2/3件",
		"二度と発火しません",
		"config.ini",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("報告に %q が含まれていない:\n%s", want, report)
		}
	}

	// 成功したジョブは報告に混ざらない
	if strings.Contains(report, "expireSweepJob") {
		t.Errorf("成功したジョブが報告に含まれている:\n%s", report)
	}
}

/*
リグレッション: 全件成功（＝正常な config.ini での起動）では
失敗扱いにならず、通知メッセージも生成されないこと。

ここが壊れると毎朝の起動ごとに誤検知のSlackエラーが飛ぶ。
*/
func TestJobRegistryReportsNothingWhenAllRegistered(t *testing.T) {
	r := &jobRegistry{}
	names := []string{
		"buyingBTCJobEveryDay", "buyingETHJobEveryDay",
		"syncBTCBuyOrderJob", "syncETHBuyOrderJob",
		"sellOrderJob", "deleteRecordJob",
		"savePriceHistoryJob(朝)", "savePriceHistoryJob(夕)",
		"sendResultsJob", "rolloverSellOrderJob",
		"expireSweepJob", "reconcileJob",
		"cancelBuyOrderJob", "gracefulShutdown",
	}
	for _, name := range names {
		r.record(name, "06:30", nil)
	}

	if r.hasFailures() {
		t.Error("全件成功なのに hasFailures() = true")
	}
	if got := r.failureReport(); got != "" {
		t.Errorf("全件成功時の failureReport() = %q, want \"\"（誤検知の通知を出さない）", got)
	}
	if got, want := r.total(), len(names); got != want {
		t.Errorf("total() = %d, want %d", got, want)
	}
	if got, want := r.registered, len(names); got != want {
		t.Errorf("registered = %d, want %d", got, want)
	}
}
