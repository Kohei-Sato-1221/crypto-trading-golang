package main

import (
	"log"

	app "github.com/Kohei-Sato-1221/crypto-trading-golang/go/app/bitflyerApp"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/config"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/models"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/utils"
)

func main() {
	log.Println("🔷Start Bifflyer Trading🔷")
	config.NewConfig()
	models.NewMysqlBase()
	utils.LogSetting(config.Config.LogFile)
	log.Printf("#######\n")
	log.Printf("config:%#v\n", config.Config)
	log.Printf("#######\n")
	log.Println()
	app.StartBfService()
}
