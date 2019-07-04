package config

import (
	"log"
	"gopkg.in/ini.v1"
	"os"
	"time"
)

type ConfigList struct {
	ApiKey      string
	ApiSecret   string
	LogFile     string
	ProductCode string
	
	TradeDuration	time.Duration
	Durations		map[string]time.Duration
	DbName			string
	SQLDriver		string
	Port				int
}

var BaseURL string

var Config ConfigList

func init() {
	cfg, err := ini.Load("config.ini")
	if err != nil {
		log.Printf("Failed to read file: %v", err)
		os.Exit(1)
	}
	pcfg, err := ini.Load("private_config.ini")
	if err != nil {
		log.Printf("Failed to read file: %v", err)
		os.Exit(1)
	}
	
	durations := map[string]time.Duration {
		"1s" : time.Second,
		"1m" : time.Minute,
		"1h" : time.Hour,
	}

	Config = ConfigList{
		ApiKey:        pcfg.Section("bitflyer").Key("api_key").String(),
		ApiSecret:     pcfg.Section("bitflyer").Key("api_secret").String(),
		LogFile:       cfg.Section("tradeSetting").Key("logfile_path").String(),
		ProductCode:   cfg.Section("tradeSetting").Key("product_code").String(),
		
		Durations: durations,
		TradeDuration: durations[cfg.Section("tradeSetting").Key("trade_duration").String()],
		DbName: cfg.Section("db").Key("name").String(),
		SQLDriver: cfg.Section("db").Key("driver").String(),
		Port: cfg.Section("web").Key("port").MustInt(),
				
	}
	
	BaseURL = pcfg.Section("bitflyer").Key("base_url").String()
}