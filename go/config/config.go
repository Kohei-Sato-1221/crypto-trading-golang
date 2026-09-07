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

	// DefaultBuyMinuteToExpire は買い注文の有効期限(分)の既定値。10080分 = 7日
	DefaultBuyMinuteToExpire = 10080

	// DefaultSellMinuteToExpire は売り注文の有効期限(分)の既定値。43200分 = 30日（Bitflyerの上限）
	DefaultSellMinuteToExpire = 43200

	// DefaultBuyOrderCancelDays は cancelBuyOrderJob が買い注文を能動キャンセルするまでの経過日数の既定値。
	// buy_minute_to_expire(10080分 = 7日)と整合させている。
	DefaultBuyOrderCancelDays = 7

	// DefaultExpireSweepGraceMinutes は expireSweepJob が「期限切れ」とみなすまでの猶予(分)の既定値。
	// アプリと取引所の時計ずれを吸収し、期限直前のレコードを早まってCANCELLEDにしないためのもの。
	DefaultExpireSweepGraceMinutes = 10

	// DefaultTriggerTime06 は expireSweepJob のスケジュール既定値(JST)。
	// EC2の稼働窓(3:00〜12:30 / 16:30〜23:00 JST)の内側かつ、買い注文ジョブ(06:30)より前に置く。
	DefaultTriggerTime06 = "06:05"

	// DefaultSellRolloverDaysBeforeExpire は rolloverSellOrderJob が売り注文を巻き直す
	// 「有効期限の何日前か」の既定値。売り注文の期限30日 − 3日 = 27日目にローリングする。
	// 3日前から対象になるため、日次ジョブで最大3回の再試行機会が確保される。
	DefaultSellRolloverDaysBeforeExpire = 3

	// DefaultSellRolloverFallbackDays は expire_date が未設定の旧レコードを
	// ローリング対象とみなすまでの経過日数の既定値（発注から27日）。
	DefaultSellRolloverFallbackDays = 27

	// DefaultSellRolloverMaxPerRun は rolloverSellOrderJob が1回の実行で処理する上限件数の既定値。
	// 1件あたり最大3リクエスト(照会/キャンセル/発注)のためレート制限の歯止めを兼ねる。
	DefaultSellRolloverMaxPerRun = 20

	// DefaultTriggerTime05 は rolloverSellOrderJob のスケジュール既定値(JST)。
	// EC2の稼働窓(3:00〜12:30 JST)の内側かつ、expireSweepJob(06:05)より前に置く。
	DefaultTriggerTime05 = "05:30"

	// DefaultTriggerTime07 は reconcileJob のスケジュール既定値(JST)。
	// 実行環境のRaspberry Piが停止する 01:30〜02:45 JST を避け、
	// 失効検出(06:05)の後・買い注文(06:30)の前に置くことで、
	// スロット解放が済んだ状態のDBと取引所を突合できるようにしている。
	DefaultTriggerTime07 = "06:15"

	// DefaultTriggerTime08 は cancelBuyOrderJob のスケジュール既定値(JST)。
	// 実行環境のRaspberry Piが停止する 01:30〜02:45 JST を避けている。
	// 旧設定は23:45だったが、EC2稼働窓の外という誤った前提に基づいていたため22:45へ移した。
	DefaultTriggerTime08 = "22:45"

	// DefaultTriggerTime09 はグレースフルシャットダウンのスケジュール既定値(JST)。
	// 実行環境のRaspberry Piが停止する 01:30 JST の直前に置き、
	// 実行中のジョブを完了させてからアプリを終了させる。
	DefaultTriggerTime09 = "01:20"

	// DefaultNoOrderAlertDays は「ボットの買い注文が何日間0件ならアラートするか」の既定値。
	DefaultNoOrderAlertDays = 3

	// DefaultBalanceDiffThresholdBTC はBTC残高の乖離を通知する閾値の既定値。
	// 最小取引単位(0.001 BTC)の半分。手数料・端数による微小なずれで鳴らないようにしている。
	DefaultBalanceDiffThresholdBTC = 0.0005

	// DefaultBalanceDiffThresholdETH はETH残高の乖離を通知する閾値の既定値。
	// 最小取引単位(0.01 ETH)の半分。
	DefaultBalanceDiffThresholdETH = 0.005
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

		DBDriver: pcfg.Section("database").Key("driver").String(),
		MySql:    pcfg.Section("database").Key("mysql").String(),
		Postgres: pcfg.Section("database").Key("postgres").String(),

		BFMaxBuy:  cfg.Section("bitflyer").Key("max_buy_orders").MustInt(),
		BFMaxSell: cfg.Section("bitflyer").Key("max_sell_orders").MustInt(),

		// 未設定(0)の場合は models.CalculateMinuteToExpire() 側で既定値(10080分=7日)が使われる
		BFBuyMinuteToExpire: cfg.Section("bitflyer").Key("buy_minute_to_expire").MustInt(DefaultBuyMinuteToExpire),

		// 売り注文の有効期限(分)。Bitflyerの上限は43200分(30日)
		BFSellMinuteToExpire: cfg.Section("bitflyer").Key("sell_minute_to_expire").MustInt(DefaultSellMinuteToExpire),

		// 未設定(0)の場合は bitflyer.MaxChildOrdersCount(実効上限500)が使われる
		BFChildOrdersCount: cfg.Section("bitflyer").Key("child_orders_count").MustInt(0),

		// cancelBuyOrderJob が能動キャンセルする経過日数。0以下ならDefaultBuyOrderCancelDays(7日)
		BFBuyOrderCancelDays: cfg.Section("bitflyer").Key("buy_order_cancel_days").MustInt(DefaultBuyOrderCancelDays),

		// expireSweepJob が失効とみなすまでの猶予(分)。0以下ならDefaultExpireSweepGraceMinutes(10分)
		BFExpireSweepGraceMinutes: cfg.Section("bitflyer").Key("expire_sweep_grace_minutes").MustInt(DefaultExpireSweepGraceMinutes),

		// rolloverSellOrderJob が売り注文を巻き直す「有効期限の何日前か」。0以下ならDefaultSellRolloverDaysBeforeExpire(3日)
		BFSellRolloverDaysBeforeExpire: cfg.Section("bitflyer").Key("sell_rollover_days_before_expire").MustInt(DefaultSellRolloverDaysBeforeExpire),

		// expire_date未設定の旧売り注文をローリング対象とみなす経過日数。0以下ならDefaultSellRolloverFallbackDays(27日)
		BFSellRolloverFallbackDays: cfg.Section("bitflyer").Key("sell_rollover_fallback_days").MustInt(DefaultSellRolloverFallbackDays),

		// rolloverSellOrderJob が1回の実行で処理する上限件数。0以下ならDefaultSellRolloverMaxPerRun(20件)
		BFSellRolloverMaxPerRun: cfg.Section("bitflyer").Key("sell_rollover_max_per_run").MustInt(DefaultSellRolloverMaxPerRun),

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
		// rolloverSellOrderJob の実行時刻(JST)。未設定ならDefaultTriggerTime05(05:30)
		TriggerTime05: NormalizeTriggerTime(cfg.Section("tradeSetting").Key("trigger_time_05").String(), DefaultTriggerTime05),
		// expireSweepJob の実行時刻(JST)。未設定ならDefaultTriggerTime06(06:05)
		TriggerTime06: NormalizeTriggerTime(cfg.Section("tradeSetting").Key("trigger_time_06").String(), DefaultTriggerTime06),
		// reconcileJob の実行時刻(JST)。未設定ならDefaultTriggerTime07(06:15)
		TriggerTime07: NormalizeTriggerTime(cfg.Section("tradeSetting").Key("trigger_time_07").String(), DefaultTriggerTime07),
		// cancelBuyOrderJob の実行時刻(JST)。未設定ならDefaultTriggerTime08(22:45)
		TriggerTime08: NormalizeTriggerTime(cfg.Section("tradeSetting").Key("trigger_time_08").String(), DefaultTriggerTime08),
		// グレースフルシャットダウンの実行時刻(JST)。未設定ならDefaultTriggerTime09(01:20)
		TriggerTime09: NormalizeTriggerTime(cfg.Section("tradeSetting").Key("trigger_time_09").String(), DefaultTriggerTime09),

		SlackAPIURL: pcfg.Section("slack").Key("api_url").String(),
		SlackToken:  pcfg.Section("slack").Key("token").String(),

		BudgetCriteria: cfg.Section("app").Key("budget_criteria").MustFloat64(500000),

		// reconcileJob がボットの発注ゼロを検知するまでの日数。0以下ならDefaultNoOrderAlertDays(3日)
		NoOrderAlertDays: cfg.Section("app").Key("no_order_alert_days").MustInt(DefaultNoOrderAlertDays),

		// 残高乖離の通知閾値。0以下なら既定値(BTC:0.0005 / ETH:0.005)
		BalanceDiffThresholdBTC: cfg.Section("app").Key("balance_diff_threshold_btc").MustFloat64(DefaultBalanceDiffThresholdBTC),
		BalanceDiffThresholdETH: cfg.Section("app").Key("balance_diff_threshold_eth").MustFloat64(DefaultBalanceDiffThresholdETH),

		// DBが追跡していない既知の保有量。残高突合のベースラインとして想定保有量に加算する（既定0）
		UntrackedHoldingBTC: cfg.Section("app").Key("untracked_holding_btc").MustFloat64(0),
		UntrackedHoldingETH: cfg.Section("app").Key("untracked_holding_eth").MustFloat64(0),

		IsTest: isTest,
	}

	BaseURL = pcfg.Section("bitflyer").Key("base_url").String()
}

type ConfigList struct {
	Exchange string

	BFMaxSell int
	BFMaxBuy  int

	BFBuyMinuteToExpire int // 買い注文の有効期限(分)。0ならmodels側の既定値(10080分=7日)

	BFSellMinuteToExpire int // 売り注文の有効期限(分)。0以下ならDefaultSellMinuteToExpire(43200分=30日)

	BFChildOrdersCount int // getchildordersの1回あたり取得件数。0ならbitflyer.MaxChildOrdersCountを使う

	BFBuyOrderCancelDays int // cancelBuyOrderJobが能動キャンセルする経過日数。0以下ならDefaultBuyOrderCancelDays(7日)

	BFExpireSweepGraceMinutes int // expireSweepJobが失効とみなすまでの猶予(分)。0以下ならDefaultExpireSweepGraceMinutes(10分)

	BFSellRolloverDaysBeforeExpire int // rolloverSellOrderJobが売り注文を巻き直す有効期限の何日前か。0以下ならDefaultSellRolloverDaysBeforeExpire(3日)

	BFSellRolloverFallbackDays int // expire_date未設定の旧売り注文をローリング対象とみなす経過日数。0以下ならDefaultSellRolloverFallbackDays(27日)

	BFSellRolloverMaxPerRun int // rolloverSellOrderJobが1回の実行で処理する上限件数。0以下ならDefaultSellRolloverMaxPerRun(20件)

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

	DBDriver string // "mysql" or "postgres" - DB切り替え用（private_config.ini [database] driver）
	MySql    string // MySQL接続DSN
	Postgres string // PostgreSQL接続DSN

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
	TriggerTime05 string // rolloverSellOrderJob の実行時刻(JST)
	TriggerTime06 string // expireSweepJob の実行時刻(JST)
	TriggerTime07 string // reconcileJob の実行時刻(JST)
	TriggerTime08 string // cancelBuyOrderJob の実行時刻(JST)
	TriggerTime09 string // グレースフルシャットダウンの実行時刻(JST)

	SlackAPIURL string
	SlackToken  string

	IsTest bool

	BudgetCriteria float64 // 日本円がこの金額以下なら買い注文をしない

	NoOrderAlertDays int // reconcileJobがボットの発注ゼロを検知するまでの日数。0以下ならDefaultNoOrderAlertDays(3日)

	BalanceDiffThresholdBTC float64 // BTC残高の乖離を通知する閾値。0以下ならDefaultBalanceDiffThresholdBTC(0.0005)
	BalanceDiffThresholdETH float64 // ETH残高の乖離を通知する閾値。0以下ならDefaultBalanceDiffThresholdETH(0.005)

	UntrackedHoldingBTC float64 // DBが追跡していない既知のBTC保有量。残高突合のベースライン（既定0）
	UntrackedHoldingETH float64 // DBが追跡していない既知のETH保有量。残高突合のベースライン（既定0）
}

var BaseURL string

var Config ConfigList
