package okex

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

var BaseURL string

// Place an Order
func (apiClient *APIClient) PlaceOrder(order *Order) (*OrderResponse, error) {
	data, err := json.Marshal(order)
	if err != nil {
		return nil, err
	}
	requestPath := "/api/spot/v3/orders"
	resp, err := apiClient.doHttpRequest("POST", requestPath, map[string]string{}, data)
	if err != nil {
		fmt.Printf("res:%s\n", resp)
		return nil, err
	}
	var response OrderResponse
	err = json.Unmarshal(resp, &response)
	if err != nil {
		fmt.Printf("error in PlaceOrder:%s  resp:%s\n", err, resp)
		return nil, err
	}
	return &response, nil
}

// Cancel an Order
func (apiClient *APIClient) CancelOrder(orderID, pair string) (*OrderResponse, error) {
	cancelOrderParam := &Order{
		InstrumentID: pair,
	}
	data, err := json.Marshal(cancelOrderParam)
	if err != nil {
		return nil, err
	}
	requestPath := "/api/spot/v3/cancel_orders/" + orderID
	resp, err := apiClient.doHttpRequest("POST", requestPath, map[string]string{}, data)
	if err != nil {
		fmt.Printf("res:%s\n", resp)
		return nil, err
	}
	var response OrderResponse
	err = json.Unmarshal(resp, &response)
	if err != nil {
		fmt.Printf("error in CancelOrder err:%s\n", response.ErrorMsg)
		return nil, err
	}
	return &response, nil
}

// GetTickerInfo
func (apiClient *APIClient) GetOkexTicker(productCode string) (*Ticker, error) {
	requestPath := "/api/spot/v3/instruments/" + productCode + "/ticker"
	resp, err := apiClient.doHttpRequest("GET", requestPath, map[string]string{}, nil)
	log.Printf("requestPath=%s resp=%s", requestPath, string(resp))
	if err != nil {
		log.Printf("action=GetOkexTicker err=%s", err.Error())
		return nil, err
	}
	var ticker Ticker
	err = json.Unmarshal(resp, &ticker)
	if err != nil {
		log.Printf("action=GetOkexTicker err=%s", err.Error())
		return nil, err
	}
	return &ticker, nil
}

// GetBalance
func (apiClient *APIClient) GetBlance(currency string) (*Balance, error) {
	requestPath := "/api/spot/v3/accounts/" + currency
	resp, err := apiClient.doHttpRequest("GET", requestPath, map[string]string{}, nil)
	log.Printf("requestPath=%s resp=%s", requestPath, string(resp))
	if err != nil {
		log.Printf("action=GetBalance err=%s", err.Error())
		return nil, err
	}
	var balance Balance
	err = json.Unmarshal(resp, &balance)
	if err != nil {
		log.Printf("action=GetBalance err=%s", err.Error())
		return nil, err
	}
	return &balance, nil
}

// GetOrderList
func (apiClient *APIClient) GetOrderList(productCode, state string) (*[]Order, error) {
	requestPath := "/api/spot/v3/orders?instrument_id=" + productCode + "&state=" + state
	resp, err := apiClient.doHttpRequest("GET", requestPath, map[string]string{}, nil)
	log.Printf("requestPath=%s resp=%s ", requestPath, string(resp))
	if err != nil {
		log.Printf("action=GetOrderList err=%s", err.Error())
		return nil, err
	}
	var orders []Order
	err = json.Unmarshal(resp, &orders)
	if err != nil {
		log.Printf("action=GetOrderList err=%s", err.Error())
		return nil, err
	}
	return &orders, nil
}

type Order struct {
	OrderID      string `json:"order_id"`
	ClientOid    string `json:"client_oid"`
	Type         string `json:"type"`
	Side         string `json:"side"`
	InstrumentID string `json:"instrument_id"`
	OrderType    string `json:"order_type"`
	Price        string `json:"price"`
	Size         string `json:"size"`
	State        string `json:"state"`
	Timestamp    string `json:"timestamp"`
}

type Ticker struct {
	BestAsk string `json:"best_ask"`
	BestBid string `json:"best_bid"`
	Ltp     string `json:"last"`
	High    string `json:"high_24h"`
	Low     string `json:"low_24h"`
}

type Balance struct {
	Balance   string `json:"balance"`
	Hold      string `json:"hold"`
	Available string `json:"available"`
	Currency  string `json:"currency"`
	ID        string `json:"id"`
}

type OrderResponse struct {
	OrderId   string `json:"order_id"`
	ClientOid string `json:"client_oid"`
	Result    bool   `json:"result"`
	ErrorCode string `json:"error_code"`
	ErrorMsg  string `json:"error_message"`
}

type APIClient struct {
	apikey     string
	apisecret  string
	passphrase string
	exchange   string
	httpClient *http.Client
}

func New(key, secret, passphrase, exchange string) *APIClient {
	if len(exchange) == 0 {
		exchange = "okex"
	}
	apiClient := &APIClient{key, secret, passphrase, exchange, &http.Client{}}
	return apiClient
}

func (apiClient APIClient) header(method, requestPath string, body []byte) map[string]string {
	timestamp := getIsoTime()
	message := timestamp + method + requestPath + string(body)

	preHashStr := getPreHashString(timestamp, method, requestPath, string(body))

	log.Printf("preHashStr:%s ", preHashStr)
	log.Printf("timeStamp:%s ", timestamp)

	log.Printf("message:%s ", message)

	sign := signBySha256Base64(preHashStr, apiClient.apisecret)
	return map[string]string{
		"OK-ACCESS-KEY":        apiClient.apikey,
		"OK-ACCESS-SIGN":       sign,
		"OK-ACCESS-TIMESTAMP":  timestamp,
		"OK-ACCESS-PASSPHRASE": apiClient.passphrase,
		"Accept":               "application/json",
		"Content-Type":         "application/json; charset=UTF-8",
	}
}

func (apiClient *APIClient) doHttpRequest(method, requestPath string, query map[string]string, data []byte) (body []byte, err error) {
	endpoint := BaseURL + requestPath
	log.Printf("action=doGETPOST endpoint=%s", endpoint)
	req, err := http.NewRequest(method, endpoint, bytes.NewBuffer(data))
	if err != nil {
		return
	}
	q := req.URL.Query()
	for key, value := range query {
		q.Add(key, value)
	}
	req.URL.RawQuery = q.Encode()

	for key, value := range apiClient.header(method, requestPath, data) {
		req.Header.Add(key, value)
	}
	resp, err := apiClient.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

/*
 Get Iso Format time
  example: 2019-08-23T18:02:48.284Z
*/
func getIsoTime() string {
	utcTime := time.Now().UTC()
	iso := utcTime.String()
	isoBytes := []byte(iso)
	iso = string(isoBytes[:10]) + "T" + string(isoBytes[11:23]) + "Z"
	return iso
}

/*
 Get Pre Hash String
 Params:
    timestamp    = 2019-08-15T11:22:2.123Z
    method       = POST
    request_path = /orders?before=2&limit=30
    body         = {"product_id":"ETH-USD","order_id":"1233455"}

  Return:
    2019-08-15T11:22:2.123ZPOST/orders?before=2&limit=30{"product_id":"ETH-USD","order_id":"1233455"}
*/
func getPreHashString(timestamp string, method string, requestPath string, body string) string {
	return timestamp + strings.ToUpper(method) + requestPath + body
}

/*
 To sign using sha256 + base64
*/
func signBySha256Base64(preHashStr, secretKey string) string {
	mac := hmac.New(sha256.New, []byte(secretKey))
	_, err := mac.Write([]byte(preHashStr))
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
