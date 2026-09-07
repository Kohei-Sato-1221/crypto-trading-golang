package bitflyer

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/config"
)

/*
F20 の回帰テスト。

http.Client にタイムアウトが無いと、取引所側がハングした場合にジョブが無期限に
ブロックする。特にローリングは「キャンセル成功 → 再発注」の間にHTTP往復があるため、
ここで止まると現物を保有したまま売り注文が無い「裸の保有」から復帰できない。
さらに gracefulShutdown は実行中ジョブを最大5分待って os.Exit(0) するため、
ハングしたジョブごとプロセスが落ちる。
*/

// NewBitflyer が返すクライアントにタイムアウトが設定されていること。
func TestNewBitflyerSetsHTTPTimeout(t *testing.T) {
	apiClient := NewBitflyer("key", "secret", 10, 20)

	if apiClient.httpClient == nil {
		t.Fatal("httpClient が nil")
	}
	if apiClient.httpClient.Timeout == 0 {
		t.Fatal("http.Client にタイムアウトが無い（取引所がハングするとジョブが無限に待つ）")
	}
	if apiClient.httpClient.Timeout != httpClientTimeout {
		t.Errorf("Timeout = %v, want %v", apiClient.httpClient.Timeout, httpClientTimeout)
	}
	// 90秒周期の同期ジョブが重ならないよう、周期より十分短いこと
	if apiClient.httpClient.Timeout > 60*time.Second {
		t.Errorf("Timeout が長すぎる: %v", apiClient.httpClient.Timeout)
	}
}

// リグレッション: タイムアウト追加でコンストラクタの他のフィールドが壊れていないこと。
func TestNewBitflyerKeepsFields(t *testing.T) {
	apiClient := NewBitflyer("key", "secret", 10, 20)

	if apiClient.apikey != "key" || apiClient.apisecret != "secret" {
		t.Errorf("APIキーが設定されていない: %+v", apiClient)
	}
	if apiClient.Max_buy_orders != 10 || apiClient.Max_sell_orders != 20 {
		t.Errorf("上限値が設定されていない: buy=%d sell=%d",
			apiClient.Max_buy_orders, apiClient.Max_sell_orders)
	}
}

/*
応答が返らないサーバに対して、無限に待たずエラーで戻ること。

リクエストが apiClient.httpClient を経由していることの確認も兼ねる
（http.DefaultClient を直接使う実装に戻すと、この短いタイムアウトが効かず
テストがタイムアウトで落ちる）。
*/
func TestDoRequestReturnsOnTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // 応答を返さずハングする取引所を模す
	}))
	defer server.Close()

	originalBaseURL := config.BaseURL
	config.BaseURL = server.URL + "/"
	defer func() { config.BaseURL = originalBaseURL }()

	apiClient := NewBitflyer("key", "secret", 1, 1)
	apiClient.httpClient.Timeout = 200 * time.Millisecond

	done := make(chan error, 1)
	go func() {
		_, _, err := apiClient.doRequest("GET", "ticker", map[string]string{"product_code": "BTC_JPY"}, nil)
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
