package config

import (
	"log"
	"gopkg.in/ini.v1"
	"os"
)

type ConfigList struct {
	ApiKey      string
	ApiSecret   string
	LogFile     string
	ProductCode string
}

var BaseURL string

var Config ConfigList

func init() {
	cfg, err := ini.Load("config.ini")
	if err != nil {
		log.Printf("Failed to read file: %v", err)
		os.Exit(1)
	}

	Config = ConfigList{
		ApiKey:        cfg.Section("bitflyer").Key("api_key").String(),
		ApiSecret:     cfg.Section("bitflyer").Key("api_secret").String(),
		LogFile:       cfg.Section("tradeSetting").Key("logfile_path").String(),
		ProductCode:   cfg.Section("tradeSetting").Key("product_code").String(),		
	}
	
	BaseURL = cfg.Section("bitflyer").Key("base_url").String()
}