package bitflyerApp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/bitflyer"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/config"
)

/*
G1 のテスト。

部分約定した売り注文の損益を後から手で記録し直すには「約定ぶんがいくらで売れたか」
＝ average_price が必須である。ところが**キャンセルした注文は取引所APIから即座に消える**
（実測確認済み: キャンセル直後の個別照会は空配列 [] を返す）ため、
キャンセル前のこの個別照会で拾い損ねると二度と取得できない。
rolloverOrderSnapshot が average_price を捨てていると手動対応が原理的に不可能になるため、
取りこぼしを検知する回帰テストを置く。
*/

// newLookupTestServer は getchildorders の応答を固定するテストサーバを立て、
// config.BaseURL をそこへ差し替える（外部へは一切リクエストが飛ばない）。
func newLookupTestServer(t *testing.T, body string) *bitflyer.APIClient {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	original := config.BaseURL
	config.BaseURL = server.URL + "/"
	t.Cleanup(func() {
		config.BaseURL = original
		server.Close()
	})
	return bitflyer.NewBitflyer("key", "secret", 1, 1)
}

/*
部分約定している注文の average_price が snapshot に保持されること。

これが 0 に落ちると、Slack通知にも載らず、キャンセル後は再取得できないため
損益の手動補正ができなくなる。
*/
func TestLookupRolloverOrderKeepsAveragePriceOnPartialFill(t *testing.T) {
	apiClient := newLookupTestServer(t, `[{
		"id": 138398,
		"child_order_acceptance_id": "JRF20260907-010131-018801",
		"product_code": "ETH_JPY",
		"side": "SELL",
		"child_order_type": "LIMIT",
		"price": 512345,
		"average_price": 511000,
		"size": 0.03,
		"child_order_state": "ACTIVE",
		"executed_size": 0.01,
		"outstanding_size": 0.02,
		"cancel_size": 0
	}]`)

	snapshot, err := lookupRolloverOrder(apiClient, "ETH_JPY", "JRF20260907-010131-018801")
	if err != nil {
		t.Fatalf("lookupRolloverOrder failed: %v", err)
	}

	if !snapshot.found {
		t.Fatal("注文が見つかっていない")
	}
	if snapshot.averagePrice == 0 {
		t.Fatal("部分約定しているのに averagePrice が 0。キャンセル後は再取得できないため手動対応が不可能になる")
	}
	if snapshot.averagePrice != 511000 {
		t.Errorf("averagePrice = %v, want 511000", snapshot.averagePrice)
	}

	// リグレッション: average_price 追加で既存フィールドが壊れていないこと
	if snapshot.state != "ACTIVE" {
		t.Errorf("state = %q, want ACTIVE", snapshot.state)
	}
	if snapshot.size != 0.03 || snapshot.executedSize != 0.01 || snapshot.outstandingSize != 0.02 {
		t.Errorf("数量が壊れている: %+v", snapshot)
	}
	if got := snapshot.remainingSize(); got != 0.02 {
		t.Errorf("remainingSize() = %v, want 0.02", got)
	}
}

// 未約定（executed_size=0）なら averagePrice は 0 のまま。
func TestLookupRolloverOrderAveragePriceZeroWhenUnfilled(t *testing.T) {
	apiClient := newLookupTestServer(t, `[{
		"child_order_acceptance_id": "JRF20260907-010131-018802",
		"product_code": "BTC_JPY",
		"price": 5075000,
		"average_price": 0,
		"size": 0.001,
		"child_order_state": "ACTIVE",
		"executed_size": 0,
		"outstanding_size": 0.001
	}]`)

	snapshot, err := lookupRolloverOrder(apiClient, "BTC_JPY", "JRF20260907-010131-018802")
	if err != nil {
		t.Fatalf("lookupRolloverOrder failed: %v", err)
	}
	if snapshot.averagePrice != 0 {
		t.Errorf("未約定なのに averagePrice = %v", snapshot.averagePrice)
	}
}

// 注文が存在しない場合はゼロ値の snapshot をエラーなしで返すこと（リグレッション）。
func TestLookupRolloverOrderNotFound(t *testing.T) {
	apiClient := newLookupTestServer(t, `[]`)

	snapshot, err := lookupRolloverOrder(apiClient, "BTC_JPY", "JRF-MISSING")
	if err != nil {
		t.Fatalf("lookupRolloverOrder failed: %v", err)
	}
	if snapshot.found {
		t.Errorf("存在しない注文が found=true になっている: %+v", snapshot)
	}
	if snapshot.averagePrice != 0 {
		t.Errorf("averagePrice = %v, want 0", snapshot.averagePrice)
	}
}
