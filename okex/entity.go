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
}

type OrderResponse struct {
	OrderID       string `json:"ordId"`
	ClientOrderID string `json:"clOrdId"`
	Tag           string `json:"tag"`
	ResultCode    string `json:"sCode"` //0 means success
	Message       string `json:"sMsg"`
}

type CancelOrderParam struct {
	OrderID       string `json:"ordId"`
	ClientOrderID string `json:"clOrdId"`
}

type CancelOrderResponse struct {
	ClientOrderID string `json:"clOrdId"`
	Tag           string `json:"tag"`
	Result        string `json:"sCode"`
	Message       string `json:"sMsg"`
}

type Ticker struct {
	BestAsk string `json:"askPx"`
	BestBid string `json:"bidPx"`
	Ltp     string `json:"last"`
	High    string `json:"high24h"`
	Low     string `json:"low24h"`
}

type Balance struct {
	Details struct {
		Balance   string `json:"cashBal"`
		Hold      string `json:"frozenBal"`
		Available string `json:"availEq"`
		Currency  string `json:"ccy"`
	} `json:"details"`
}

type APIClient struct {
	apikey     string
	apisecret  string
	passphrase string
	exchange   string
	httpClient *http.Client
}
