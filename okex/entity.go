package okex

import "net/http"

type Order struct {
	OrderID       string `json:"ordId"`
	ClientOrderID string `json:"clOrdId"`
	TradeMode     string `json:"tdMode"`
	InstrumentID  string `json:"instId"`
	Side          string `json:"side"`
	OrderType     string `json:"ordType"`
	Size          string `json:"sz"`
	Price         string `json:"px"`
	Tag           string `json:"tag"`
	Timestamp     string `json:"cTime"`
}

type OrderResponse struct {
	Code    string `json:"code"`
	Message string `json:"msg"`
	Data    []struct {
		OrderID       string `json:"ordId"`
		ClientOrderID string `json:"clOrdId"`
		Tag           string `json:"tag"`
		ResultCode    string `json:"sCode"` //0 means success
		Message       string `json:"sMsg"`
	} `json:"data"`
}

type GetOrderListRes struct {
	Code    string  `json:"code"`
	Message string  `json:"msg"`
	Data    []Order `json:"data"`
}

type CancelOrderParam struct {
	OrderID       string `json:"ordId"`
	ClientOrderID string `json:"clOrdId"`
}

type CancelOrderResponse struct {
	Code    string `json:"code"`
	Message string `json:"msg"`
	Data    struct {
		OrderID       string `json:"ordId"`
		ClientOrderID string `json:"clOrdId"`
		Tag           string `json:"tag"`
		Result        string `json:"sCode"`
		Message       string `json:"sMsg"`
	} `json:"data"`
}

type Ticker struct {
	BestAsk string `json:"askPx"`
	BestBid string `json:"bidPx"`
	Ltp     string `json:"last"`
	High    string `json:"high24h"`
	Low     string `json:"low24h"`
}

type GetTickerRes struct {
	Code    string   `json:"code"`
	Message string   `json:"msg"`
	Data    []Ticker `json:"data"`
}

type Balance struct {
	Balance   string `json:"cashBal"`
	Hold      string `json:"frozenBal"`
	Available string `json:"availEq"`
	Currency  string `json:"ccy"`
}

type GetBalanceRes struct {
	Code    string `json:"code"`
	Message string `json:"msg"`
	Data    struct {
		AdjustedEquity string    `json:"adjEq"`
		Details        []Balance `json:"details"`
	} `json:"data"`
}

type APIClient struct {
	apikey     string
	apisecret  string
	passphrase string
	exchange   string
	httpClient *http.Client
}
