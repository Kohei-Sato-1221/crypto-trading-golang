package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

const baseURL = "https://slack.com/api/chat.postMessage"

type APIClient struct {
	token      string
	channel    string
	httpClient *http.Client
}

func NewSlack(token, channel string) *APIClient {
	apiClient := &APIClient{token, channel,&http.Client{}}
	return apiClient
}

//TODO 他のクライアントと共通化すること
func (apiClient *APIClient) doGETPOST(method string, query map[string]string, data []byte) (body []byte, err error) {
	req, err := http.NewRequest(method, baseURL, bytes.NewBuffer(data))
	if err != nil {
		return
	}
	q := req.URL.Query()
	for key, value := range query {
		q.Add(key, value)
	}
	req.URL.RawQuery = q.Encode()

	header := map[string]string{
		"Content-Type": "application/json;charset=utf-8",
		"Authorization": "Bearer " + apiClient.token,
	}
	for key, value := range header {
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

type postMessageParams struct {
	Channel string `json:"channel"`
	Text    string `json:"text"`
}


func (apiClient *APIClient) PostMessage(text string) error {
	params := postMessageParams {
		Channel: apiClient.channel,
		Text:    text,
	}
	data, err := json.Marshal(params)
	fmt.Println(string(data))
	if err != nil {
		return err
	}
	resp, err := apiClient.doGETPOST("POST", map[string]string{}, data)
	fmt.Println(string(resp))
	if err != nil {
		fmt.Printf("res:%s\n", resp)
		return err
	}
	if err != nil {
		fmt.Printf("err:%s\n", err)
		return err
	}
	return nil
}
