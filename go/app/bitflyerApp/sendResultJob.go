package bitflyerApp

import (
	"fmt"
	"log"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/bitflyer"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

func sendResultsJob(apiClient *bitflyer.APIClient) {
	log.Println("【sendResultsJob】Start of job")
	results, err := models.GetResults()
	if err != nil {
		errMsg := fmt.Sprintf("【ERROR】Failed to get results: %v", err)
		log.Printf("%s\n", errMsg)
		slackClient.PostMessage(errMsg, true)
		log.Println("【sendResultsJob】End of job as error")
		return
	}

	// 各通貨の残高を取得
	balances, err := apiClient.GetBalances()
	if err != nil {
		errMsg := fmt.Sprintf("【ERROR】Failed to get JPY balance: %v", err)
		log.Printf("%s\n", errMsg)
		slackClient.PostMessage(errMsg, true)
		log.Println("【sendResultsJob】End of job as error")
		return
	}

	var jpyAvailable, jpyBalance float64
	var btcAvailable, btcBalance float64
	var ethAvailable, ethBalance float64
	for _, balance := range balances {
		if balance.CurrentCode == "JPY" {
			jpyAvailable = balance.Available
			jpyBalance = balance.Amount
		}
		if balance.CurrentCode == "BTC" {
			btcAvailable = balance.Available
			btcBalance = balance.Amount
		}
		if balance.CurrentCode == "ETH" {
			ethAvailable = balance.Available
			ethBalance = balance.Amount
		}
	}

	// 結果に残高を追加して送信
	balanceText := fmt.Sprintf("【現在の資産残高(利用可能/総額)】\nJPY   %.f/%.f\nBTC   %.5f/%.5f\nETH   %.5f/%.5f",
		jpyAvailable, jpyBalance, btcAvailable, btcBalance, ethAvailable, ethBalance)

	resultsWithBalance := fmt.Sprintf("%s\n\n%s", results, balanceText)
	slackClient.PostMessage(resultsWithBalance, false)
	log.Printf("【sendResultsJob】JPY balance: %.2f", jpyBalance)
	log.Println("【sendResultsJob】End of job")
}
