package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

const baseURL = "https://slack.com/api/chat.postMessage"

type APIClient struct {
	token      string
	channel    string
	errChannel string
	apiURL     string
	httpClient *http.Client
}

func NewSlack(token, channel, errChannel, apiURL string) *APIClient {
	apiClient := &APIClient{token, channel, errChannel, apiURL, &http.Client{}}
	return apiClient
}

func (apiClient *APIClient) PostMessage(text string, isError bool) error {
	apiURL := apiClient.apiURL
	if apiURL != "" {
		return apiClient.sendMessageToSlackV2(text)
	} else {
		return apiClient.sendMessageToSlackV1(text, isError)
	}
}

// deprecated
func (apiClient *APIClient) sendMessageToSlackV1(text string, isError bool) error {
	var channelName string
	if isError {
		channelName = apiClient.errChannel
	} else {
		channelName = apiClient.channel
	}
	params := postMessageParams{
		Channel: channelName,
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

// SendMessageToSlack Slackにメッセージを送信する
func (apiClient *APIClient) sendMessageToSlackV2(message string) (err error) {
	apiURL := apiClient.apiURL
	p, err := json.Marshal(payload{Text: message})
	if err != nil {
		return err
	}
	resp, err := http.PostForm(apiURL, url.Values{"payload": {string(p)}})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

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
		"Content-Type":  "application/json;charset=utf-8",
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

type payload struct {
	Text string `json:"text"`
}
