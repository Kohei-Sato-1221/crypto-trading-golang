package bitflyerApp

import "testing"

/*
F12 の回帰テスト。

部分約定した売り注文の残数量は size - executed_size で求めるが、float64 の減算では
0.03 - 0.02 = 0.009999999999999998 となり、ETH の最小取引単位 0.01 を僅差で下回る。
そのまま比較すると「残数量が最小取引単位未満」としてローリングが毎日スキップされ続け、
いずれ期限切れで裸の保有になる。残数量は必ず小数8桁へ丸めること。
*/

// 桁ノイズが落ち、最小取引単位との比較が正しく行えること。
func TestRoundRolloverSize(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want float64
	}{
		{name: "0.03-0.02 の減算誤差", in: 0.03 - 0.02, want: 0.01},
		{name: "0.3-0.1 の減算誤差", in: 0.3 - 0.1, want: 0.2},
		{name: "0.001-0.0005", in: 0.001 - 0.0005, want: 0.0005},
		{name: "誤差のない値はそのまま", in: 0.02, want: 0.02},
		{name: "ゼロ", in: 0, want: 0},
		{name: "8桁は保持する", in: 0.00000001, want: 0.00000001},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := roundRolloverSize(tt.in); got != tt.want {
				t.Errorf("roundRolloverSize(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// CANCELED 済み（outstanding_size=0）の経路で残数量が最小取引単位を下回らないこと。
func TestRemainingSizeCancelledPathHasNoFloatingNoise(t *testing.T) {
	const ethMinSize = 0.01

	snapshot := rolloverOrderSnapshot{
		found:           true,
		state:           "CANCELED",
		size:            0.03,
		executedSize:    0.02,
		outstandingSize: 0,
	}

	// 修正前は 0.009999999999999998 となり ethMinSize を下回っていた
	if raw := snapshot.size - snapshot.executedSize; raw >= ethMinSize {
		t.Skipf("この環境では減算誤差が発生しないためスキップ: %v", raw)
	}

	got := snapshot.remainingSize()
	if got != ethMinSize {
		t.Errorf("remainingSize() = %v, want %v", got, ethMinSize)
	}
	if got < ethMinSize {
		t.Errorf("残数量が最小取引単位を下回っている（正当なローリングが永久にスキップされる）: %v", got)
	}
}

// リグレッション: ACTIVE な注文では outstanding_size がそのまま残数量になること。
func TestRemainingSizeUsesOutstandingSizeWhenPositive(t *testing.T) {
	snapshot := rolloverOrderSnapshot{
		found:           true,
		state:           "ACTIVE",
		size:            0.05,
		executedSize:    0.02,
		outstandingSize: 0.03,
	}
	if got := snapshot.remainingSize(); got != 0.03 {
		t.Errorf("remainingSize() = %v, want 0.03", got)
	}
}

// リグレッション: 全量約定済みなら残数量は0で、ローリングはスキップ側へ倒れること。
func TestRemainingSizeFullyExecutedIsZero(t *testing.T) {
	snapshot := rolloverOrderSnapshot{
		found:           true,
		state:           "COMPLETED",
		size:            0.02,
		executedSize:    0.02,
		outstandingSize: 0,
	}
	if got := snapshot.remainingSize(); got != 0 {
		t.Errorf("remainingSize() = %v, want 0", got)
	}
}

// リグレッション: 部分約定していない ACTIVE 注文は発注時の数量がそのまま残ること。
func TestRemainingSizeNoPartialFill(t *testing.T) {
	snapshot := rolloverOrderSnapshot{
		found:           true,
		state:           "ACTIVE",
		size:            0.001,
		executedSize:    0,
		outstandingSize: 0.001,
	}
	if got := snapshot.remainingSize(); got != 0.001 {
		t.Errorf("remainingSize() = %v, want 0.001", got)
	}
}
