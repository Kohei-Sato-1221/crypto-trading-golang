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
	State         string `json:"state"`
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

func (l *GetOrderListRes) getSpecifiedInstOrderList(instrumentID, state string) *[]Order {
	var orders []Order
	for _, order := range l.Data {
		if order.InstrumentID == instrumentID && order.State == state {
			orders = append(orders, order)
		}
	}
	return &orders
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
	Currency  string `json:"ccy"`
	Balance   string `json:"cashBal"`
	Hold      string `json:"frozenBal"`
	Available string `json:"availEq"`
}

type GetBalanceRes struct {
	Code    string `json:"code"`
	Message string `json:"msg"`
	Data    []struct {
		AdjustedEquity string    `json:"adjEq"`
		Details        []Balance `json:"details"`
	} `json:"data"`
}

func (b *GetBalanceRes) getSpecifiedCcyBalance(currency string) *Balance {
	var balance Balance
	if len(b.Data) < 1 {
		return &balance
	}
	for _, data := range b.Data[0].Details {
		if data.Currency == currency {
			balance = data
			return &balance
		}
	}
	return &balance
}

type APIClient struct {
	apikey     string
	apisecret  string
	passphrase string
	exchange   string
	httpClient *http.Client
}
