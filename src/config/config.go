package config

import (
	"log"
	"gopkg.in/ini.v1"
	"os"
	"time"
)

type ConfigList struct {
	Exchange        string
	
	ApiKey          string
	ApiSecret       string
	LogFile         string
	ProductCode     string
	
	OKApiKey        string
	OKApiSecret     string
	OKPassPhrase    string
	
	TradeDuration	time.Duration
	Durations		map[string]time.Duration
	DbName			string
	SQLDriver		string
	Port			int
	ParallelOrders  int
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
		OKApiKey:      pcfg.Section("okex").Key("api_key").String(),
		OKApiSecret:   pcfg.Section("okex").Key("api_secret").String(),
		OKPassPhrase:  pcfg.Section("okex").Key("pass_phrase").String(),

		LogFile:       cfg.Section("tradeSetting").Key("logfile_path").String(),
		ProductCode:   cfg.Section("tradeSetting").Key("product_code").String(),	
		Exchange:      cfg.Section("exchange").Key("exchange").String(),
		
		Durations: durations,
		TradeDuration: durations[cfg.Section("tradeSetting").Key("trade_duration").String()],
		DbName: cfg.Section("db").Key("name").String(),
		SQLDriver: cfg.Section("db").Key("driver").String(),
		Port: cfg.Section("web").Key("port").MustInt(),
		ParallelOrders: cfg.Section("tradeSetting").Key("parallel_orders").MustInt(),
	}
	
	BaseURL = pcfg.Section("bitflyer").Key("base_url").String()
}