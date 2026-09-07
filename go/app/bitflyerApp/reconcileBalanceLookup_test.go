package bitflyerApp

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/bitflyer"
)

/*
F15 の回帰テスト。

残高レスポンスを map に詰めて exchangeAmounts[target.Currency] で引いていたため、
レスポンスに BTC / ETH が含まれない場合に map のゼロ値 0 が「実残高0」として扱われ、
必ず「不足」側の🚨アラートになっていた。方向は安全側だが、原因の分からない
誤アラートが毎日鳴り続ける。存在有無を判別できる形にして、無ければ突合をスキップする。

実機確認: 2026-09-07 に GET /v1/me/getbalance を実行し、BTC / ETH を含む
40件超の通貨エントリが返ることを確認済み（現状は欠落していない・防御的な修正）。
*/

func balanceResponse() []bitflyer.Balance {
	return []bitflyer.Balance{
		{CurrentCode: "JPY", Amount: 123456, Available: 100000},
		{CurrentCode: "BTC", Amount: 0.0123, Available: 0.0023},
		{CurrentCode: "ETH", Amount: 0.45, Available: 0.05},
	}
}

// 対象通貨があれば Amount（拘束分を含む総保有量）を返すこと（リグレッション）。
func TestLookupExchangeBalanceFound(t *testing.T) {
	tests := []struct {
		currency string
		want     float64
	}{
		{currency: "BTC", want: 0.0123},
		{currency: "ETH", want: 0.45},
		{currency: "JPY", want: 123456},
	}
	for _, tt := range tests {
		got, found := lookupExchangeBalance(balanceResponse(), tt.currency)
		if !found {
			t.Errorf("%s が見つからない", tt.currency)
			continue
		}
		if got != tt.want {
			t.Errorf("%s = %v, want %v（Available ではなく Amount を返すこと）", tt.currency, got, tt.want)
		}
	}
}

// 対象通貨が無いときは found=false になり、ゼロ値を実残高として扱わないこと。
func TestLookupExchangeBalanceNotFound(t *testing.T) {
	balances := []bitflyer.Balance{{CurrentCode: "JPY", Amount: 123456}}

	got, found := lookupExchangeBalance(balances, "ETH")
	if found {
		t.Fatalf("ETH は含まれていないので found=false の想定: got=%v", got)
	}
	if got != 0 {
		t.Errorf("見つからない場合の値は0の想定: %v", got)
	}

	// 空レスポンス・nil でも同様（0を実残高として扱うと必ず不足アラートになる）
	if _, found := lookupExchangeBalance(nil, "BTC"); found {
		t.Error("nil レスポンスで found=true になっている")
	}
	if _, found := lookupExchangeBalance([]bitflyer.Balance{}, "BTC"); found {
		t.Error("空レスポンスで found=true になっている")
	}
}

// 残高が0のエントリが存在する場合は「実残高0」として突合すること（見つからないと混同しない）。
func TestLookupExchangeBalanceZeroAmountIsFound(t *testing.T) {
	balances := []bitflyer.Balance{{CurrentCode: "ETH", Amount: 0, Available: 0}}

	got, found := lookupExchangeBalance(balances, "ETH")
	if !found {
		t.Fatal("エントリが存在するので found=true の想定（残高0は正当な突合対象）")
	}
	if got != 0 {
		t.Errorf("Amount = %v, want 0", got)
	}
}

// 通知には通貨コードのみを載せ、残高の金額を含めないこと。
func TestBalanceCurrencyListHasNoAmounts(t *testing.T) {
	text := balanceCurrencyList(balanceResponse())

	for _, want := range []string{"JPY", "BTC", "ETH"} {
		if !strings.Contains(text, want) {
			t.Errorf("通貨コード %q が含まれていない: %s", want, text)
		}
	}
	for _, secret := range []string{"123456", "0.0123", "0.45"} {
		if strings.Contains(text, secret) {
			t.Errorf("残高の金額が通知に含まれている: %s", text)
		}
	}
}

// 通貨数が多い場合は reconcileSampleSize 件で切り詰めること。
func TestBalanceCurrencyListTruncates(t *testing.T) {
	balances := make([]bitflyer.Balance, 0, reconcileSampleSize+5)
	for i := 0; i < reconcileSampleSize+5; i++ {
		balances = append(balances, bitflyer.Balance{CurrentCode: fmt.Sprintf("CUR%d", i)})
	}

	text := balanceCurrencyList(balances)
	if !strings.Contains(text, "...他5件") {
		t.Errorf("切り詰めの表示が無い: %s", text)
	}
	if got := strings.Count(text, "CUR"); got != reconcileSampleSize {
		t.Errorf("列挙件数 = %d, want %d: %s", got, reconcileSampleSize, text)
	}
}
