package bitbank

import (
	"net/http"
	"testing"
	"time"
)

/*
F6 の回帰テスト。

GetBBTicker は data が null のレスポンスでも ticker01.Data.Sell を参照しており、
bitbank 側の障害・エラー応答でそのまま panic していた。
戻り値は買い注文の指値算出に使われるため、値が得られない場合は必ず error を返し、
呼び出し側が「発注しない」側に倒せる必要がある。
*/

const validBody = `{"success":1,"data":{"sell":"15000001","buy":"14999999","high":"15200000",` +
	`"low":"14800000","last":"15000000","vol":"123.4567","timestamp":1757000000000}}`

func TestParseBBTickerValid(t *testing.T) {
	ticker, err := parseBBTicker("btc_jpy", []byte(validBody))
	if err != nil {
		t.Fatalf("parseBBTicker failed: %v", err)
	}
	if ticker == nil {
		t.Fatal("ticker must not be nil")
	}
	if ticker.Last != 15000000 || ticker.Low != 14800000 {
		t.Errorf("last=%v low=%v, want 15000000 / 14800000", ticker.Last, ticker.Low)
	}
	if ticker.Sell != 15000001 || ticker.Buy != 14999999 || ticker.High != 15200000 {
		t.Errorf("sell=%v buy=%v high=%v", ticker.Sell, ticker.Buy, ticker.High)
	}
	if ticker.Timestamp != 1757000000000 {
		t.Errorf("timestamp=%v", ticker.Timestamp)
	}
	// vol は小数を含むため整数化に失敗するが、価格算出には使わないので best-effort で0のまま通す
	if ticker.Vol != 0 {
		t.Errorf("vol=%v, want 0 (best-effort)", ticker.Vol)
	}
}

func TestParseBBTickerInvalidResponses(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"data が null（旧実装は nil 参照で panic）", `{"success":0,"data":null}`},
		{"data キーが無い", `{"success":0}`},
		{"JSONとして壊れている", `<html>503 Service Unavailable</html>`},
		{"空ボディ", ``},
		{"価格が数値でない", `{"success":1,"data":{"sell":"-","buy":"-","high":"-","low":"-","last":"-","vol":"-","timestamp":0}}`},
		{"last が空文字", `{"success":1,"data":{"sell":"1","buy":"1","high":"1","low":"1","last":"","vol":"1","timestamp":0}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticker, err := parseBBTicker("btc_jpy", []byte(tt.body))
			if err == nil {
				t.Fatalf("expected an error, got ticker=%+v", ticker)
			}
			if ticker != nil {
				t.Errorf("ticker must be nil on error, got %+v", ticker)
			}
		})
	}
}

/*
F20 の回帰テスト。

http.Get はタイムアウトを持たない http.DefaultClient を使うため、bitbank 側が
ハングすると GetBBTicker が無期限にブロックし、買い注文ジョブごと止まる。
タイムアウト付きの httpClient を経由していることを確認する。
*/

// パッケージのHTTPクライアントにタイムアウトが設定されていること。
func TestHTTPClientHasTimeout(t *testing.T) {
	if httpClient == nil {
		t.Fatal("httpClient が nil")
	}
	if httpClient.Timeout == 0 {
		t.Fatal("http.Client にタイムアウトが無い（bitbank がハングするとジョブが無限に待つ）")
	}
	if httpClient.Timeout != httpClientTimeout {
		t.Errorf("Timeout = %v, want %v", httpClient.Timeout, httpClientTimeout)
	}
}

/*
応答が返らないサーバに対して、無限に待たずエラーで戻ること。

GetBBTicker が httpClient を経由していることの確認も兼ねる
（http.Get に戻すと、このテスト用クライアントの短いタイムアウトが効かず落ちる）。
*/
func TestGetBBTickerReturnsOnTimeout(t *testing.T) {
	original := httpClient
	httpClient = &http.Client{
		Timeout:   200 * time.Millisecond,
		Transport: hangingTransport{},
	}
	defer func() { httpClient = original }()

	done := make(chan error, 1)
	go func() {
		_, err := GetBBTicker("btc_jpy")
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("ハングしたサーバに対して err=nil が返った")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("タイムアウトが効かずリクエストがブロックし続けている")
	}
}

// hangingTransport は応答を返さず、リクエストのキャンセルだけを待つ RoundTripper。
type hangingTransport struct{}

func (hangingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	<-req.Context().Done()
	return nil, req.Context().Err()
}
