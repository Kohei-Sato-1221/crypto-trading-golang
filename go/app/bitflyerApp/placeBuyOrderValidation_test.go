package bitflyerApp

import (
	"errors"
	"testing"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/bitbank"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/bitflyer"
)

/*
F6 の回帰テスト。

placeBuyOrder は `ticker, _ := apiClient.GetTicker(productCode)` と error を捨てており、
取得に失敗すると nil のまま ticker.Ltp を参照して panic していた。
異常時は「発注しない」側に倒す必要がある（ルート CLAUDE.md > 取引安全性）。
*/

func validBitflyerTicker() *bitflyer.Ticker {
	return &bitflyer.Ticker{ProductCode: "ETH_JPY", Ltp: 500000, BestBid: 499000, BestAsk: 501000}
}

func validBitbankTicker() *bitbank.ReturnTicker {
	return &bitbank.ReturnTicker{Last: 15000000, Low: 14800000, Sell: 15000001, Buy: 14999999}
}

func TestValidateBuyPriceSources(t *testing.T) {
	apiErr := errors.New("connection reset by peer")

	tests := []struct {
		name      string
		strategy  int
		ticker    *bitflyer.Ticker
		tickerErr error
		bbTicker  *bitbank.ReturnTicker
		bbErr     error
		wantErr   bool
	}{
		// --- 正常系（回帰）: 従来どおり発注に進めること ---
		{name: "BTC戦略で両方揃っていれば発注できる", strategy: 3,
			ticker: validBitflyerTicker(), bbTicker: validBitbankTicker(), wantErr: false},
		{name: "ETH戦略(>=10)で両方揃っていれば発注できる", strategy: 10001,
			ticker: validBitflyerTicker(), bbTicker: validBitbankTicker(), wantErr: false},
		{name: "ETH戦略はbitbankが取得できなくても発注できる", strategy: 10001,
			ticker: validBitflyerTicker(), bbTicker: nil, bbErr: apiErr, wantErr: false},
		{name: "7日安値戦略(20001)も同様", strategy: 20001,
			ticker: validBitflyerTicker(), bbTicker: nil, bbErr: apiErr, wantErr: false},

		// --- 異常系: 発注しない側に倒すこと ---
		{name: "GetTickerがエラーなら発注しない", strategy: 10001,
			ticker: nil, tickerErr: apiErr, wantErr: true},
		{name: "GetTickerがnilを返したら発注しない（旧実装のpanic箇所）", strategy: 10001,
			ticker: nil, wantErr: true},
		{name: "errorはnilでもtickerがnilなら発注しない", strategy: 3,
			ticker: nil, bbTicker: validBitbankTicker(), wantErr: true},
		{name: "Ltpが0なら発注しない", strategy: 10001,
			ticker: &bitflyer.Ticker{Ltp: 0, BestBid: 499000}, wantErr: true},
		{name: "BestBidが0なら発注しない", strategy: 10001,
			ticker: &bitflyer.Ticker{Ltp: 500000, BestBid: 0}, wantErr: true},
		{name: "BTC戦略でbitbankがエラーなら発注しない", strategy: 3,
			ticker: validBitflyerTicker(), bbTicker: nil, bbErr: apiErr, wantErr: true},
		{name: "BTC戦略でbitbankがnilなら発注しない", strategy: 3,
			ticker: validBitflyerTicker(), bbTicker: nil, wantErr: true},
		{name: "BTC戦略でbitbankのlastが0なら発注しない", strategy: 3,
			ticker: validBitflyerTicker(), bbTicker: &bitbank.ReturnTicker{Last: 0, Low: 14800000}, wantErr: true},
		{name: "BTC戦略でbitbankのlowが0なら発注しない", strategy: 3,
			ticker: validBitflyerTicker(), bbTicker: &bitbank.ReturnTicker{Last: 15000000, Low: 0}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBuyPriceSources(tt.strategy, "ETH_JPY", tt.ticker, tt.tickerErr, tt.bbTicker, tt.bbErr)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateBuyPriceSources() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
