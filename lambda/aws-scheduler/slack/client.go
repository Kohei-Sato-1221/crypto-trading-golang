package slack

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
)

type payload struct {
	Text string `json:"text"`
}

func SendMessage(message string) (err error) {
	apiURL := os.Getenv("SLACK_URL")
	if apiURL == "" {
		return nil // SLACK_URLが設定されていない場合は何もしない
	}

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
