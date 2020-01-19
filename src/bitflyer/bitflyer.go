package bitflyer

import (
	"fmt"
	"crypto/hmac"
	"crypto/sha256"
	"log"
	"net/http"
	"net/url"
	"bytes"
	"strconv"
	"time"
	"config"
	"io/ioutil"
	"encoding/json"
	"encoding/hex"
	"github.com/gorilla/websocket"
)

const baseURL = "https://api.bitflyer.com/v1/"


type APIClient struct{
	apikey           string
	apisecret        string
	Max_buy_orders   int
	Max_sell_orders  int
	httpClient       *http.Client
}


func New(key, secret string, max_buy_orders, max_sell_orders int) *APIClient {
	apiClient := &APIClient{key, secret, max_buy_orders, max_sell_orders, &http.Client{}}
	return apiClient
}

func (apiClient APIClient) header(method, endpoint string, body []byte) map[string]string{
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

func (apiClient *APIClient) doGETPOST(method, urlPath string, query map[string]string, data []byte) (body []byte, err error){
	baseURL, err := url.Parse(config.BaseURL)
	if err != nil{
		return
	}
	apiURL, err := url.Parse(urlPath)
	if err != nil{
		return
	}
	endpoint := baseURL.ResolveReference(apiURL).String()
	req, err := http.NewRequest(method, endpoint, bytes.NewBuffer(data))
	if err != nil{
		return
	}
	q := req.URL.Query()
	for key, value := range query {
		q.Add(key, value)
	}
	req.URL.RawQuery = q.Encode()
	
	for key, value := range apiClient.header(method, req.URL.RequestURI(), data){
		req.Header.Add(key, value)
	}
	resp, err := apiClient.httpClient.Do(req)
	if err != nil{
		return nil, err
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil{
		return nil, err
	}
	return body, nil
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
	if err != nil{
		log.Printf("action=GetBalance err=%s", err.Error())
		return nil, err
	}
	var balance []Balance
	err = json.Unmarshal(resp, &balance)
	if err != nil{
		log.Printf("action=GetBalance err=%s", err.Error())
		return nil, err
	}
	return balance, nil
}

func (apiClient *APIClient) GetOrderByOrderId(orderId string) (*Order, error) {
	url := "me/getchildorders"
	params := make(map[string]string)
	params["child_order_acceptance_id"] = orderId
	params["product_code"] = "BTC_JPY"
	params["child_order_state"] = "COMPLETED"
	resp, err := apiClient.doGETPOST("GET", url, params, nil)
	log.Printf("url=%s resp=%s", url, string(resp))
	if err != nil{
		log.Printf("action=GetOrderByOrderId err=%s", err.Error())
		return nil, err
	}
	var orders []Order
	err = json.Unmarshal(resp, &orders)
	if err != nil{
		log.Printf("action=GetOrderByOrderId err=%s, orderId:%s", err.Error(), orderId)
		return nil, err
	}
	
	if len(orders) == 0 {
		log.Printf("action=GetOrderByOrderId No Order correspond to orderId:%s", orderId)
		return nil, nil
	}
	return &orders[0], nil
}

// to get list of unfilled(active) orders from bitflyer
func (apiClient *APIClient) GetActiveOrders() (*[]Order, error) {
	url := "me/getchildorders"
	params := make(map[string]string)
	params["product_code"] = "BTC_JPY"
	params["child_order_state"] = "ACTIVE"
	resp, err := apiClient.doGETPOST("GET", url, params, nil)
	log.Printf("url=%s resp=%s", url, string(resp))
	if err != nil{
		log.Printf("action=GetOrderByOrderId err=%s", err.Error())
		return nil, err
	}
	var orders []Order
	err = json.Unmarshal(resp, &orders)
	return &orders, nil
}


// easy to convert json to struct with https://mholt.github.io/json-to-go/
type Ticker struct {
	ProductCode     string  `json:"product_code"`
	Timestamp       string  `json:"timestamp"`
	TickID          int     `json:"tick_id"`
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
	if err != nil{
		log.Printf("action=GetBalance err=%s", err.Error())
		return nil, err
	}
	var ticker Ticker
	err = json.Unmarshal(resp, &ticker)
	if err != nil{
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


func (apiClient *APIClient) GetRealTimeTicker(symbol string, ch chan <- Ticker){
	u := url.URL{Scheme: "wss", Host: "ws.lightstream.bitflyer.com", Path: "/json-rpc"}
	log.Printf("connecting to %s", u.String())
	
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil{
		log.Println("dial:", err)
	}
	defer c.Close()
	
	channel := fmt.Sprintf("lightning_ticker_%s", symbol)
	if err := c.WriteJSON(&JsonRPC2{Version: "2.0", Method: "subscribe", Params: &SubscribeParams{channel}}); err != nil {
		log.Println("subscribe:", err)
		return
	}
	
	OUTER:
		for{
			message := new(JsonRPC2)
			if err := c.ReadJSON(message); err != nil{
				log.Println("read:", err)
				return
			}
			
			if message.Method == "channelMessage" {
				switch v := message.Params.(type){
					case map[string]interface{}:
					for key, binary := range v {
						if key == "message" {
							marshaTic, err := json.Marshal(binary)
							if err != nil {
								continue OUTER
							}
							var ticker Ticker
							if err := json.Unmarshal(marshaTic, &ticker); err != nil {
								continue OUTER
							}
							ch <- ticker
						}	
					}
				}
			}
		}
}

type Order struct {
	ID                     int     `json:"id"`
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
	OrderId string `json:"child_order_acceptance_id"`
}

func (apiClient *APIClient) PlaceOrder(order *Order) (*PlaceOrderResponse, error) {
	data, err := json.Marshal(order)
	if err != nil {
		return nil, err
	}
	url := "me/sendchildorder"
	resp, err := apiClient.doGETPOST("POST", url, map[string]string{}, data)
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

func (apiClient *APIClient) CancelOrder(order *Order) (error) {
	data, err := json.Marshal(order)
	if err != nil {
		return err
	}
	url := "me/cancelchildorder"
	resp, err := apiClient.doGETPOST("POST", url, map[string]string{}, data)
	if err != nil {
		fmt.Printf("res:%s\n", resp)
		return err
	}
	return nil
}

//func (apiClient *APIClient) GetFilledBuyOrderInfo() ([]Order, error){
//	query := map[string]string {"child_order_state":"COMPLETED"}
//	return GetOrderInfo(query)
//}
//
//
//func (apiClient *APIClient) GetOrderInfo(query map[string]string) ([]Order, error){
//	resp, err := apiClient.doGETPOST("GET", "me/getchildorders", query, nil)
//	if err != nil {
//		return nil, err
//	}
//	var ordersList []Order
//	err = json.Unmarshal(resp, &ordersList)
//	if err != nil {
//		return nil, err
//	}
//	return ordersList, nil
//}








