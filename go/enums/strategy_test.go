package enums

import "testing"

/*
F19 の回帰テスト。

旧戦略 Stg0BtcLtp3low7 は `= iota` で値が 0 のため、botStrategies に含めると
IsBotStrategy(0) が true になる。buy_orders.strategy のゼロ値（未設定・NULL・
スキャン失敗）も 0 なので、これを「ボット発注」と数えると reconcileJob の
発注ゼロ検知を取りこぼし、ボットが止まっていることに気づけなくなる。
*/

// strategy カラムのゼロ値をボット発注と誤認しないこと。
func TestIsBotStrategyRejectsZeroValue(t *testing.T) {
	if IsBotStrategy(StrategyZeroValue) {
		t.Error("IsBotStrategy(0) が true。strategy カラムのゼロ値がボット発注と誤認され、発注ゼロ検知を取りこぼす")
	}
	if Stg0BtcLtp3low7 != 0 {
		t.Fatalf("前提が変わっている: Stg0BtcLtp3low7 = %d, want 0", Stg0BtcLtp3low7)
	}
	if IsBotStrategy(Stg0BtcLtp3low7) {
		t.Error("Stg0BtcLtp3low7 は 0 でゼロ値と区別できないため false を返すこと")
	}
}

// リグレッション: 現行・旧のボット戦略は従来どおり true を返すこと。
func TestIsBotStrategyAcceptsBotStrategies(t *testing.T) {
	botValues := []int{
		StrategyLTP99, StrategyLTP98, StrategyLTP95, StrategyLTP97,
		StrategyLtpLowestIn7days5t5, StrategyLtpLowestIn7days2t8, StrategyLtpLowestIn7days7t3,
		Stg1BtcLtp997, Stg2BtcLtp98, Stg3BtcLtp90,
		Stg10EthLtp995, Stg11EthLtp98, Stg12EthLtp97, Stg13EthLtp3low7, Stg14EthLtp90,
	}
	for _, strategy := range botValues {
		if !IsBotStrategy(strategy) {
			t.Errorf("IsBotStrategy(%d) = false, want true", strategy)
		}
	}
}

// リグレッション: ボット発注でない戦略値は従来どおり false を返すこと。
func TestIsBotStrategyRejectsNonBotStrategies(t *testing.T) {
	nonBotValues := []int{
		StrategyUnknown,          // 99: DBのデフォルト値（戦略未記録）
		StrategySaturatedUnknown, // 127: 旧MySQL tinyint の飽和で判別不能
		StrategyManual,           // 90001: 取引所で手動発注し syncBuyOrders が取り込んだもの
		TEST_STG,                 // -1
		12345,                    // 未定義値
	}
	for _, strategy := range nonBotValues {
		if IsBotStrategy(strategy) {
			t.Errorf("IsBotStrategy(%d) = true, want false", strategy)
		}
	}
}

// リグレッション: 利確率のマッピングは 0 の扱いを変えても影響を受けないこと。
func TestSellProfitRateUnaffectedByZeroValueExclusion(t *testing.T) {
	// Stg0BtcLtp3low7(=0) はもともと sellProfitRates に無く、既定値へフォールバックする
	if rate, ok := SellProfitRate(Stg0BtcLtp3low7); ok || rate != SellProfitRateDefault {
		t.Errorf("SellProfitRate(0) = (%v, %v), want (%v, false)", rate, ok, SellProfitRateDefault)
	}
	if rate, ok := SellProfitRate(StrategyLTP99); !ok || rate != 1.02 {
		t.Errorf("SellProfitRate(StrategyLTP99) = (%v, %v), want (1.02, true)", rate, ok)
	}
}
