package bitflyer

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/config"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/utils"
)

/*
httpClientTimeout は取引所APIへのHTTPリクエストのタイムアウト。

タイムアウトが無いと、取引所側がハングした場合にジョブが無期限にブロックする。
特にローリングは「キャンセル成功 → 再発注」の間にHTTP往復があるため、ここで止まると
現物を保有したまま売り注文が無い「裸の保有」から復帰できない。さらに gracefulShutdown は
実行中ジョブを最大5分待って os.Exit(0) するため、ハングしたジョブごとプロセスが落ちる。
日次ジョブの実行間隔（最短90秒周期の同期ジョブ）に対して十分短い値を設定する。
*/
const httpClientTimeout = 30 * time.Second

type APIClient struct {
	apikey          string
	apisecret       string
	Max_buy_orders  int
	Max_sell_orders int
	httpClient      *http.Client
}

func NewBitflyer(key, secret string, max_buy_orders, max_sell_orders int) *APIClient {
	apiClient := &APIClient{key, secret, max_buy_orders, max_sell_orders,
		&http.Client{Timeout: httpClientTimeout}}
	return apiClient
}

func (apiClient APIClient) header(method, endpoint string, body []byte) map[string]string {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	message := timestamp + method + endpoint + string(body)

	mac := hmac.New(sha256.New, []byte(apiClient.apisecret))
	mac.Write([]byte(message))
	sign := hex.EncodeToString(mac.Sum(nil))
	return map[string]string{
		"ACCESS-KEY":       apiClient.apikey,
		"ACCESS-TIMESTAMP": timestamp,
		"ACCESS-SIGN":      sign,
		"Content-Type":     "application/json",
	}
}

// doRequest はBitflyer APIへリクエストを送り、レスポンスボディとHTTPステータスコードを返す。
// キャンセル等の更新系APIでは成否の判定にステータスコードが必要なため、
// ボディのみを返す doGETPOST から切り出している。
func (apiClient *APIClient) doRequest(method, urlPath string, query map[string]string, data []byte) (body []byte, statusCode int, err error) {
	baseURL, err := url.Parse(config.BaseURL)
	if err != nil {
		return nil, 0, err
	}
	apiURL, err := url.Parse(urlPath)
	if err != nil {
		return nil, 0, err
	}
	endpoint := baseURL.ResolveReference(apiURL).String()
	req, err := http.NewRequest(method, endpoint, bytes.NewBuffer(data))
	if err != nil {
		return nil, 0, err
	}
	q := req.URL.Query()
	for key, value := range query {
		q.Add(key, value)
	}
	req.URL.RawQuery = q.Encode()

	for key, value := range apiClient.header(method, req.URL.RequestURI(), data) {
		req.Header.Add(key, value)
	}
	resp, err := apiClient.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

// doGETPOST は doRequest の薄いラッパ。HTTPステータスコードを必要としない既存の呼び出し元向け。
func (apiClient *APIClient) doGETPOST(method, urlPath string, query map[string]string, data []byte) (body []byte, err error) {
	body, _, err = apiClient.doRequest(method, urlPath, query, data)
	return body, err
}

type Balance struct {
	CurrentCode string  `json:"currency_code"`
	Amount      float64 `json:"amount"`
	Available   float64 `json:"available"`
}

func (apiClient *APIClient) GetBalance() ([]Balance, error) {
	url := "me/getbalance"
	resp, err := apiClient.doGETPOST("GET", url, map[string]string{}, nil)
	log.Printf("url=%s resp=%s", url, string(resp))
	if err != nil {
		log.Printf("action=GetBalance err=%s", err.Error())
		return nil, err
	}
	var balance []Balance
	err = json.Unmarshal(resp, &balance)
	if err != nil {
		log.Printf("action=GetBalance err=%s", err.Error())
		return nil, err
	}
	return balance, nil
}

// GetJPYBalance returns the available JPY balance
func (apiClient *APIClient) GetJPYBalance() (float64, error) {
	balances, err := apiClient.GetBalance()
	if err != nil {
		return 0, err
	}
	for _, balance := range balances {
		if balance.CurrentCode == "JPY" {
			return balance.Available, nil
		}
	}
	return 0, nil
}

func (apiClient *APIClient) GetBalances() ([]Balance, error) {
	balances, err := apiClient.GetBalance()
	if err != nil {
		return nil, err
	}
	return balances, nil
}

const (
	// MaxChildOrdersCount は getchildorders の count の実効上限。
	// 公式ドキュメントに上限の記載がないため、実測値を定数化している。
	// 501 / 1000 / 5000 / 10000 を指定しても黙って500件で頭打ちになることを実測で確認済み。
	MaxChildOrdersCount = 500

	// MaxChildOrdersPages は before ページングで遡る最大ページ数（無限ループ防止）。
	MaxChildOrdersPages = 10

	// childOrdersPath は子注文一覧APIのパス。
	childOrdersPath = "me/getchildorders"
)

// GetChildOrdersParams は /v1/me/getchildorders のクエリパラメータ。
type GetChildOrdersParams struct {
	ProductCode            string // 必須
	ChildOrderState        string // "ACTIVE" / "COMPLETED" / "CANCELED" / "" (全状態)
	Count                  int    // 0 なら設定値(child_orders_count)、それも未設定なら MaxChildOrdersCount
	Before                 int    // 0 なら未指定（ページング用: 前ページの最小 ID）
	After                  int    // 0 なら未指定
	ChildOrderAcceptanceID string // 個別照会用
}

// resolveChildOrdersCount は実際に count へ指定する件数を決める。
// 実効上限(MaxChildOrdersCount)を超える値は取引所側で黙って切り詰められるため、こちら側でも丸める。
func resolveChildOrdersCount(requested int) int {
	if requested <= 0 {
		requested = config.Config.BFChildOrdersCount
	}
	if requested <= 0 || requested > MaxChildOrdersCount {
		requested = MaxChildOrdersCount
	}
	return requested
}

// parseChildOrderDate は child_order_date / expire_date を UTC の time.Time としてパースする。
// パースできない場合はゼロ値と error を返す。
// 実装は utils.ParseBitflyerTime に委譲する（同期処理・DB保存側と同じ解釈を保証するため）。
func parseChildOrderDate(value string) (time.Time, error) {
	return utils.ParseBitflyerTime(value)
}

// GetChildOrders は子注文一覧を1ページぶん（最大 MaxChildOrdersCount 件）取得する。
// 売買方向(side)ではフィルタしないため BUY / SELL の両方が返る点に注意すること。
func (apiClient *APIClient) GetChildOrders(p GetChildOrdersParams) ([]Order, error) {
	params := make(map[string]string)
	if p.ProductCode != "" {
		params["product_code"] = p.ProductCode
	}
	if p.ChildOrderState != "" {
		params["child_order_state"] = p.ChildOrderState
	}
	params["count"] = strconv.Itoa(resolveChildOrdersCount(p.Count))
	if p.Before > 0 {
		params["before"] = strconv.Itoa(p.Before)
	}
	if p.After > 0 {
		params["after"] = strconv.Itoa(p.After)
	}
	if p.ChildOrderAcceptanceID != "" {
		params["child_order_acceptance_id"] = p.ChildOrderAcceptanceID
	}

	resp, statusCode, err := apiClient.doRequest("GET", childOrdersPath, params, nil)
	if err != nil {
		log.Printf("action=GetChildOrders productCode=%s state=%s err=%s", p.ProductCode, p.ChildOrderState, err.Error())
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		err = fmt.Errorf("action=GetChildOrders productCode=%s state=%s statusCode=%d body=%s",
			p.ProductCode, p.ChildOrderState, statusCode, string(resp))
		log.Println(err.Error())
		return nil, err
	}
	var orders []Order
	if err := json.Unmarshal(resp, &orders); err != nil {
		log.Printf("action=GetChildOrders productCode=%s state=%s err=%s body=%s",
			p.ProductCode, p.ChildOrderState, err.Error(), string(resp))
		return nil, err
	}
	log.Printf("action=GetChildOrders productCode=%s state=%s before=%d fetched=%d",
		p.ProductCode, p.ChildOrderState, p.Before, len(orders))
	return orders, nil
}

// GetChildOrdersAll は before ページングで最大 MaxChildOrdersPages ページ遡って子注文を取得する。
// before には前ページの最小 id を渡す（Bitflyerは id < before のレコードを返す）。
// 2つ目の返り値は取得できた中で最も古い child_order_date（遡り限界の判定に使う。該当なしならゼロ値）。
func (apiClient *APIClient) GetChildOrdersAll(productCode, state string) ([]Order, time.Time, error) {
	var allOrders []Order
	var oldest time.Time
	count := resolveChildOrdersCount(0)
	before := 0

	for page := 0; page < MaxChildOrdersPages; page++ {
		orders, err := apiClient.GetChildOrders(GetChildOrdersParams{
			ProductCode:     productCode,
			ChildOrderState: state,
			Count:           count,
			Before:          before,
		})
		if err != nil {
			return nil, time.Time{}, err
		}
		if len(orders) == 0 {
			break
		}

		minID := int64(0)
		for _, order := range orders {
			allOrders = append(allOrders, order)
			if minID == 0 || order.ID < minID {
				minID = order.ID
			}
			orderDate, parseErr := parseChildOrderDate(order.ChildOrderDate)
			if parseErr != nil {
				log.Printf("action=GetChildOrdersAll orderId=%s %s", order.ChildOrderAcceptanceID, parseErr.Error())
				continue
			}
			if oldest.IsZero() || orderDate.Before(oldest) {
				oldest = orderDate
			}
		}

		// 1ページの上限に満たなければ最終ページ
		if len(orders) < count {
			break
		}
		// before を決められない場合は同じページを取り続けないよう打ち切る
		if minID <= 0 {
			log.Printf("action=GetChildOrdersAll productCode=%s state=%s no valid id for paging. stop paging", productCode, state)
			break
		}
		before = int(minID)
	}
	log.Printf("action=GetChildOrdersAll productCode=%s state=%s total=%d oldestChildOrderDate=%s",
		productCode, state, len(allOrders), oldest.Format(time.RFC3339))
	return allOrders, oldest, nil
}

// GetChildOrderByAcceptanceID は child_order_acceptance_id を指定して子注文を1件照会する。
// 該当が無い場合は (nil, nil) を返す。
//
// ⚠️ このメソッドの用途は「生きている注文（ACTIVE / COMPLETED / CANCELED）の存在確認」に限定すること。
// 失効した注文は Bitflyer の API から完全に消える（実測確認済み: child_order_state=EXPIRED は常に0件、
// 失効注文を child_order_acceptance_id で個別指定しても0件）。
// したがって nil が返っても「失効した」とは断定できず、失効判定にこのメソッドを使ってはならない。
// 失効判定にはDBに保持した expire_date を使うこと。
func (apiClient *APIClient) GetChildOrderByAcceptanceID(productCode, acceptanceID string) (*Order, error) {
	orders, err := apiClient.GetChildOrders(GetChildOrdersParams{
		ProductCode:            productCode,
		ChildOrderAcceptanceID: acceptanceID,
		Count:                  1,
	})
	if err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		log.Printf("action=GetChildOrderByAcceptanceID productCode=%s orderId=%s no order found", productCode, acceptanceID)
		return nil, nil
	}
	return &orders[0], nil
}

// easy to convert json to struct with https://mholt.github.io/json-to-go/
type Ticker struct {
	ProductCode     string  `json:"product_code"`
	Timestamp       string  `json:"timestamp"`
	TickID          int64   `json:"tick_id"`
	BestBid         float64 `json:"best_bid"`
	BestAsk         float64 `json:"best_ask"`
	BestBidSize     float64 `json:"best_bid_size"`
	BestAskSize     float64 `json:"best_ask_size"`
	TotalBidDepth   float64 `json:"total_bid_depth"`
	TotalAskDepth   float64 `json:"total_ask_depth"`
	Ltp             float64 `json:"ltp"`
	Volume          float64 `json:"volume"`
	VolumeByProduct float64 `json:"volume_by_product"`
}

func (t *Ticker) GetMiddlePrice() float64 {
	return (t.BestBid + t.BestAsk) / 2
}

func (t *Ticker) DateTime() time.Time {
	dateTime, err := time.Parse(time.RFC3339, t.Timestamp)
	if err != nil {
		log.Printf("action=DateTime, err=%s", err.Error())
	}
	return dateTime
}

func (t *Ticker) TruncateDateTime(duration time.Duration) time.Time {
	return t.DateTime().Truncate(duration)
}

func (apiClient *APIClient) GetTicker(productCode string) (*Ticker, error) {
	url := "ticker"
	resp, err := apiClient.doGETPOST("GET", url, map[string]string{"product_code": productCode}, nil)
	log.Printf("url=%s resp=%s", url, string(resp))
	if err != nil {
		log.Printf("action=GetBalance err=%s", err.Error())
		return nil, err
	}
	var ticker Ticker
	err = json.Unmarshal(resp, &ticker)
	if err != nil {
		log.Printf("action=GetBalance err=%s", err.Error())
		return nil, err
	}
	return &ticker, nil
}

type JsonRPC2 struct {
	Version string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	Result  interface{} `json:"result,omitempty"`
	Id      *int        `json:"id,omitempty"`
}

type SubscribeParams struct {
	Channel string `json:"channel"`
}

type Order struct {
	ID                     int64   `json:"id"`
	ChildOrderAcceptanceID string  `json:"child_order_acceptance_id"`
	ProductCode            string  `json:"product_code"`
	ChildOrderType         string  `json:"child_order_type"`
	Side                   string  `json:"side"`
	Price                  float64 `json:"price"`
	Size                   float64 `json:"size"`
	MinuteToExpires        int     `json:"minute_to_expire"`
	TimeInForce            string  `json:"time_in_force"`
	Status                 string  `json:"status"`
	ErrorMessage           string  `json:"error_message"`
	AveragePrice           float64 `json:"average_price"`
	ChildOrderState        string  `json:"child_order_state"`
	ExpireDate             string  `json:"expire_date"`
	ChildOrderDate         string  `json:"child_order_date"`
	OutstandingSize        float64 `json:"outstanding_size"`
	CancelSize             float64 `json:"cancel_size"`
	ExecutedSize           float64 `json:"executed_size"`
	TotalCommission        float64 `json:"total_commission"`
	Count                  int     `json:"count"`
	Before                 int     `json:"before"`
	After                  int     `json:"after"`
}

type PlaceOrderResponse struct {
	OrderId      string      `json:"child_order_acceptance_id"`
	Status       int         `json:"status"`
	ErrorMessage string      `json:"error_message"`
	Data         interface{} `json:"data"`
}

func (apiClient *APIClient) PlaceOrder(order *Order) (*PlaceOrderResponse, error) {
	data, err := json.Marshal(order)
	fmt.Println(string(data))
	if err != nil {
		fmt.Printf("err:%s\n", err)
		return nil, err
	}
	url := "me/sendchildorder"
	resp, err := apiClient.doGETPOST("POST", url, map[string]string{}, data)
	fmt.Println(string(resp))
	if err != nil {
		fmt.Printf("res:%s\n", resp)
		return nil, err
	}
	var response PlaceOrderResponse
	err = json.Unmarshal(resp, &response)
	if err != nil {
		fmt.Printf("err:%s\n", err)
		return nil, err
	}
	return &response, nil
}

type CancelOrderResponse struct {
	OrderId string `json:"child_order_acceptance_id"`
}

// APIError は Bitflyer がエラー時に返す JSON。
type APIError struct {
	Status       int         `json:"status"`
	ErrorMessage string      `json:"error_message"`
	Data         interface{} `json:"data"`
}

// CancelOrder は注文をキャンセルする。
//
// 成功: HTTP 2xx かつ ボディが空 または {"status":0}
// 失敗: HTTP 非2xx、または status != 0 / error_message != ""
// パース不能なレスポンスは「失敗」として扱う（フェイルセーフ: 成否不明のまま再発注させないため）。
func (apiClient *APIClient) CancelOrder(order *Order) error {
	data, err := json.Marshal(order)
	if err != nil {
		log.Printf("action=CancelOrder productCode=%s orderId=%s err=%s",
			order.ProductCode, order.ChildOrderAcceptanceID, err.Error())
		return err
	}
	url := "me/cancelchildorder"
	resp, statusCode, err := apiClient.doRequest("POST", url, map[string]string{}, data)
	if err != nil {
		log.Printf("action=CancelOrder productCode=%s orderId=%s err=%s",
			order.ProductCode, order.ChildOrderAcceptanceID, err.Error())
		return fmt.Errorf("failed to cancel order. productCode=%s orderId=%s err=%w",
			order.ProductCode, order.ChildOrderAcceptanceID, err)
	}
	if statusCode < 200 || statusCode >= 300 {
		return fmt.Errorf("failed to cancel order. productCode=%s orderId=%s statusCode=%d body=%s",
			order.ProductCode, order.ChildOrderAcceptanceID, statusCode, string(resp))
	}

	body := bytes.TrimSpace(resp)
	if len(body) == 0 {
		// 空ボディ + 2xx はキャンセル成功
		return nil
	}

	var apiErr APIError
	if err := json.Unmarshal(body, &apiErr); err != nil {
		// パース不能なレスポンスは失敗側に倒す
		return fmt.Errorf("failed to parse cancel order response. productCode=%s orderId=%s statusCode=%d body=%s err=%w",
			order.ProductCode, order.ChildOrderAcceptanceID, statusCode, string(body), err)
	}
	if apiErr.Status != 0 || apiErr.ErrorMessage != "" {
		return fmt.Errorf("failed to cancel order. productCode=%s orderId=%s status=%d errorMessage=%s",
			order.ProductCode, order.ChildOrderAcceptanceID, apiErr.Status, apiErr.ErrorMessage)
	}
	return nil
}
