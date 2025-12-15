package config

import (
	"log"
	"os"
	"time"

	"gopkg.in/ini.v1"
)

const (
	ConfigPath        = "config.ini"
	PrivateConfigPath = "private_config.ini"
)

func NewConfig() {
	cfg, err := ini.Load(ConfigPath)
	if err != nil {
		log.Printf("Failed to read file: %v", err)
		os.Exit(1)
	}
	pcfg, err := ini.Load(PrivateConfigPath)
	if err != nil {
		log.Printf("Failed to read file: %v", err)
		os.Exit(1)
	}

	durations := map[string]time.Duration{
		"1s": time.Second,
		"1m": time.Minute,
		"1h": time.Hour,
	}

	isTest, err := cfg.Section("app").Key("is_test").Bool()
	if err != nil {
		isTest = false
	}

	Config = ConfigList{
		Exchange: cfg.Section("app").Key("exchange").String(),

		ApiKey:    pcfg.Section("bitflyer").Key("api_key").String(),
		ApiSecret: pcfg.Section("bitflyer").Key("api_secret").String(),

		OKApiKey:     pcfg.Section("okex").Key("api_key").String(),
		OKApiSecret:  pcfg.Section("okex").Key("api_secret").String(),
		OKPassPhrase: pcfg.Section("okex").Key("pass_phrase").String(),

		OKJApiKey:     pcfg.Section("okj").Key("api_key").String(),
		OKJApiSecret:  pcfg.Section("okj").Key("api_secret").String(),
		OKJPassPhrase: pcfg.Section("okj").Key("pass_phrase").String(),

		MySql: pcfg.Section("database").Key("mysql").String(),

		BFMaxBuy:  cfg.Section("bitflyer").Key("max_buy_orders").MustInt(),
		BFMaxSell: cfg.Section("bitflyer").Key("max_sell_orders").MustInt(),

		BFBTCBuyAmount01: cfg.Section("bitflyer").Key("btc_buy_amount_01").MustFloat64(),
		BFBTCBuyAmount02: cfg.Section("bitflyer").Key("btc_buy_amount_02").MustFloat64(),
		BFBTCBuyAmount03: cfg.Section("bitflyer").Key("btc_buy_amount_03").MustFloat64(),

		BFETHBuyAmount01: cfg.Section("bitflyer").Key("eth_buy_amount_01").MustFloat64(),
		BFETHBuyAmount02: cfg.Section("bitflyer").Key("eth_buy_amount_02").MustFloat64(),
		BFETHBuyAmount03: cfg.Section("bitflyer").Key("eth_buy_amount_03").MustFloat64(),

		OKMaxBuy:  cfg.Section("okex").Key("max_buy_orders").MustInt(),
		OKMaxSell: cfg.Section("okex").Key("max_sell_orders").MustInt(),

		OKBTCBuyAmount01: cfg.Section("okex").Key("btc_buy_amount_01").MustFloat64(),
		OKBTCBuyAmount02: cfg.Section("okex").Key("btc_buy_amount_02").MustFloat64(),
		OKBTCBuyAmount03: cfg.Section("okex").Key("btc_buy_amount_03").MustFloat64(),

		OKETHBuyAmount01: cfg.Section("okex").Key("eth_buy_amount_01").MustFloat64(),
		OKETHBuyAmount02: cfg.Section("okex").Key("eth_buy_amount_02").MustFloat64(),
		OKETHBuyAmount03: cfg.Section("okex").Key("eth_buy_amount_03").MustFloat64(),

		LogFile:     cfg.Section("tradeSetting").Key("logfile_path").String(),
		ProductCode: cfg.Section("tradeSetting").Key("product_code").String(),

		Durations:      durations,
		TradeDuration:  durations[cfg.Section("tradeSetting").Key("trade_duration").String()],
		DbName:         cfg.Section("db").Key("name").String(),
		SQLDriver:      cfg.Section("db").Key("driver").String(),
		Port:           cfg.Section("web").Key("port").MustInt(),
		ParallelOrders: cfg.Section("tradeSetting").Key("parallel_orders").MustInt(),

		TriggerTime01: cfg.Section("tradeSetting").Key("trigger_time_01").String(),
		TriggerTime02: cfg.Section("tradeSetting").Key("trigger_time_02").String(),
		TriggerTime03: cfg.Section("tradeSetting").Key("trigger_time_03").String(),
		TriggerTime04: cfg.Section("tradeSetting").Key("trigger_time_04").String(),

		SlackAPIURL: pcfg.Section("slack").Key("api_url").String(),
		SlackToken:  pcfg.Section("slack").Key("token").String(),

		BudgetCriteria: cfg.Section("app").Key("budget_criteria").MustFloat64(500000),

		IsTest: isTest,
	}

	BaseURL = pcfg.Section("bitflyer").Key("base_url").String()
}

type ConfigList struct {
	Exchange string

	BFMaxSell int
	BFMaxBuy  int

	BFBTCBuyAmount01 float64
	BFBTCBuyAmount02 float64
	BFBTCBuyAmount03 float64

	BFETHBuyAmount01 float64
	BFETHBuyAmount02 float64
	BFETHBuyAmount03 float64

	OKMaxBuy  int
	OKMaxSell int

	OKBTCBuyAmount01 float64
	OKBTCBuyAmount02 float64
	OKBTCBuyAmount03 float64

	OKETHBuyAmount01 float64
	OKETHBuyAmount02 float64
	OKETHBuyAmount03 float64

	ApiKey      string
	ApiSecret   string
	LogFile     string
	ProductCode string

	MySql string

	OKApiKey     string
	OKApiSecret  string
	OKPassPhrase string

	OKJApiKey     string
	OKJApiSecret  string
	OKJPassPhrase string

	TradeDuration  time.Duration
	Durations      map[string]time.Duration
	DbName         string
	SQLDriver      string
	Port           int
	ParallelOrders int

	TriggerTime01 string
	TriggerTime02 string
	TriggerTime03 string
	TriggerTime04 string

	SlackAPIURL string
	SlackToken  string

	IsTest bool

	BudgetCriteria float64 // 日本円がこの金額以下なら買い注文をしない
}

var BaseURL string

var Config ConfigList
