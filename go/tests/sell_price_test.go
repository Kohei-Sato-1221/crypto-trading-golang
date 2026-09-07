package tests

import (
	"testing"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/enums"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

/*
TestCalculateSellOrderPrice は戦略別の利確率マッピングの網羅性を検証する。

DBに実在する全戦略値（現行のBuyPriceStrategy / 旧BTCStrategy・ETHStrategy /
未知・手動）を列挙し、マッピングに載っていない値が必ず既定値(1.015)へ
フォールバックすることを担保する。価格計算は壊れても気づきにくいため。
*/
func TestCalculateSellOrderPrice(t *testing.T) {
	const buyPrice = 1000000.0

	tests := []struct {
		name     string
		strategy int
		want     float64
	}{
		// 現行の買い戦略
		{"LTP99(-1%)は+2%", enums.StrategyLTP99, 1020000},
		{"LTP98(-2%)は+3%", enums.StrategyLTP98, 1030000},
		{"LTP97(-3%)は+5%", enums.StrategyLTP97, 1050000},
		{"LTP95(-5%・廃止)は+5%", enums.StrategyLTP95, 1050000},
		{"7日安値5:5は+5%", enums.StrategyLtpLowestIn7days5t5, 1050000},
		{"7日安値7:3は+5%", enums.StrategyLtpLowestIn7days7t3, 1050000},
		{"7日安値2:8(廃止)は+5%", enums.StrategyLtpLowestIn7days2t8, 1050000},

		// 旧戦略。ltp*0.90 の深指値のみ従来から+3%で、その挙動を維持する
		{"旧Stg3(BTC ltp*0.90)は+3%", enums.Stg3BtcLtp90, 1030000},
		{"旧Stg14(ETH ltp*0.90)は+3%", enums.Stg14EthLtp90, 1030000},

		// 旧戦略の浅い指値・未知の戦略値はフォールバック(+1.5%)
		{"旧Stg0はフォールバック", enums.Stg0BtcLtp3low7, 1015000},
		{"旧Stg1はフォールバック", enums.Stg1BtcLtp997, 1015000},
		{"旧Stg2はフォールバック", enums.Stg2BtcLtp98, 1015000},
		{"旧Stg10はフォールバック", enums.Stg10EthLtp995, 1015000},
		{"旧Stg11はフォールバック", enums.Stg11EthLtp98, 1015000},
		{"旧Stg12はフォールバック", enums.Stg12EthLtp97, 1015000},
		{"旧Stg13はフォールバック", enums.Stg13EthLtp3low7, 1015000},
		{"未記録(99)はフォールバック", enums.StrategyUnknown, 1015000},
		{"飽和(127)はフォールバック", enums.StrategySaturatedUnknown, 1015000},
		{"手動(90001)はフォールバック", enums.StrategyManual, 1015000},
		{"未定義値はフォールバック", 12345, 1015000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := models.BuyOrderInfo{Price: buyPrice, Strategy: tt.strategy}
			if got := info.CalculateSellOrderPrice(); got != tt.want {
				t.Errorf("strategy=%d: sellPrice = %.2f, want %.2f", tt.strategy, got, tt.want)
			}
		})
	}
}

// TestSellProfitRateFallbackFlag はフォールバックしたかのフラグ（ログ表示に使う）を検証する。
func TestSellProfitRateFallbackFlag(t *testing.T) {
	known := models.BuyOrderInfo{Price: 1000000, Strategy: enums.StrategyLTP99}
	if rate, ok := known.SellProfitRate(); !ok || rate != 1.02 {
		t.Errorf("StrategyLTP99: rate=%v known=%v, want rate=1.02 known=true", rate, ok)
	}

	unknown := models.BuyOrderInfo{Price: 1000000, Strategy: enums.StrategyUnknown}
	if rate, ok := unknown.SellProfitRate(); ok || rate != enums.SellProfitRateDefault {
		t.Errorf("StrategyUnknown: rate=%v known=%v, want rate=%v known=false", rate, ok, enums.SellProfitRateDefault)
	}
}
