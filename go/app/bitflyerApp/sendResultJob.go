package bitflyerApp

import (
	"fmt"
	"log"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
)

func sendResultsJob() {
	log.Println("【sendResultsJob】Start of job")
	results, err := models.GetResults()
	if err != nil {
		errMsg := fmt.Sprintf("【ERROR】Failed to get results: %v", err)
		log.Printf("%s\n", errMsg)
		slackClient.PostMessage(errMsg, true)
		log.Println("【sendResultsJob】End of job as error")
		return
	}
	slackClient.PostMessage(results, false)
	log.Println("【sendResultsJob】End of job")
}
