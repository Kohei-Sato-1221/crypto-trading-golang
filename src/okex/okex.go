package okex

import (
	"net/http"
	"fmt"
)

const baseUrl = "https://www.okex.com/"

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

func (apiClient *APIClient) ShowParams() {
	fmt.Printf("ex: %s %s %s", apiClient.apikey, apiClient.apisecret, apiClient.passphrase)
}