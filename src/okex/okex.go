package okex

import (
	"net/http"
//	"net/url"
	"fmt"
//	"strconv"
	"time"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"encoding/hex"
	"io/ioutil"
	"log"
)

const okexBaseURL = "https://www.okex.com"

type APIClient struct{
	apikey     string
	apisecret  string
	passphrase string
	httpClient *http.Client
}

func New(key, secret, passphrase string) *APIClient {
	apiClient := &APIClient{key, secret, passphrase, &http.Client{}}
	return apiClient
}

func (apiClient APIClient) header(method, requestPath string, body []byte) map[string]string{
//	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	timestamp := time.Now().Format("2006-01-02T15:04:05.123Z")
	message := timestamp + method + requestPath + string(body)
	
	log.Printf("message:%s ", message)
	
	mac := hmac.New(sha256.New, []byte(apiClient.apisecret))
	mac.Write([]byte(message))
	sign := hex.EncodeToString(mac.Sum(nil))
	return map[string]string{
		"OK-ACCESS-KEY":       apiClient.apikey,
		"OK-ACCESS-SIGN":      sign,
		"OK-ACCESS-TIMESTAMP": timestamp,
		"OK-ACCESS-PASSPHRASE": apiClient.passphrase,
		"Content-Type":     "application/json",
	}
}

type Order struct {
	ClientOid      string  `json:"client_oid"`
	Type           string  `json:"type"`
	Side           string  `json:"side"`
	InstrumentId   string  `json:"instrument_id"`
	OrderType      string  `json:"order_type"`
	Price          string  `json:"price"`
	Size           string  `json:"size"`
} 

type PlaceOrderResponse struct {
	OrderId    string `json:"order_id"`
	ClientOid  string `json:"client_oid"`
	Result     bool   `json:"result"`
	ErrorCode  string `json:"error_code"`
	ErrorMsg   string `json:"error_message"`
}

func (apiClient *APIClient) PlaceOrder(order *Order) (*PlaceOrderResponse, error) {
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
	var response PlaceOrderResponse
	err = json.Unmarshal(resp, &response)
	if err != nil {
		fmt.Printf("error in PlaceOrder:%s  resp:%s\n", err, resp)
		return nil, err
	}
	return &response, nil
}


func (apiClient *APIClient) doHttpRequest(method, requestPath string, query map[string]string, data []byte) (body []byte, err error){
	endpoint :=  okexBaseURL + requestPath
	log.Printf("action=doGETPOST endpoint=%s", endpoint)
	req, err := http.NewRequest(method, endpoint, bytes.NewBuffer(data))
	if err != nil{
		return
	}
	q := req.URL.Query()
	for key, value := range query {
		q.Add(key, value)
	}
	req.URL.RawQuery = q.Encode()
	
	for key, value := range apiClient.header(method, requestPath, data){
		log.Printf("k:%s v:%s", key, value)
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

func (apiClient *APIClient) ShowParams() {
	fmt.Printf("ex: %s %s %s", apiClient.apikey, apiClient.apisecret, apiClient.passphrase)
}