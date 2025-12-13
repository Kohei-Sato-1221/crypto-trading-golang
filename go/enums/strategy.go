package enums

// 20251213からはBuyPriceStrategyを利用
type BuyPriceStrategy int

const (
	StrategyLTP99 = 10001
	StrategyLTP98 = 10002
	StrategyLTP95 = 10003

	StrategyLtpLowestIn7days5t5 = 20001
	StrategyLtpLowestIn7days2t8 = 20002
)

// 以下は古い戦略2：0251213からはBuyPriceStrategyを利用
type BTCStrategy int

const TEST_STG = -1

const (
	Stg0BtcLtp3low7 = iota
	Stg1BtcLtp997
	Stg2BtcLtp98
	Stg3BtcLtp90
)

// 以下は古い戦略2：0251213からはBuyPriceStrategyを利用
type ETHStrategy int

const (
	Stg10EthLtp995 = iota + 10
	Stg11EthLtp98
	Stg12EthLtp97
	Stg13EthLtp3low7
	Stg14EthLtp90
)
