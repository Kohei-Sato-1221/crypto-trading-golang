package controller

import (
	"config"
	"log"
	"okex"
	//"runtime"
)

func StartOKEXService() {
	log.Println("【StartOKEXService】")
	apiClient := okex.New(config.Config.OKApiKey, config.Config.OKApiSecret, config.Config.OKPassPhrase)	
	apiClient.ShowParams()
	//runtime.Goexit()
}




