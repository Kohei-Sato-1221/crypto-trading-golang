package enums

type BTCStrategy int

const TEST_STG = -1

const (
	Stg0BtcLtp3low7 = iota
	Stg1BtcLtp997
	Stg2BtcLtp98
	Stg3BtcLtp90
)

type ETHStrategy int

const (
	Stg10EthLtp995 = iota + 10
	Stg11EthLtp98
	Stg12EthLtp97
	Stg13EthLtp3low7
	Stg14EthLtp90
)
