package utils

import (
	"log"
	"math"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/go/enums"
)

func Round(f float64) float64 {
	return math.Floor(f + .5)
}

func CalculateBuyPrice(ltp, low float64, strategy int, lowestPriceInPast7DaysFromDB *float64) float64 {
	log.Printf("LTP:%10.2f  BestBid:%10.2f ", ltp, low)
	if strategy == enums.TEST_STG {
		return Round(ltp * 0.8)
	}

	lowestPriceInPast7Days := Round(ltp * 0.90)
	if strategy >= 20001 && lowestPriceInPast7DaysFromDB != nil {
		// DBから取得した過去7日間の最低価格を使用
		lowestPriceInPast7Days = *lowestPriceInPast7DaysFromDB
		log.Printf("Using lowest price in past 7 days from DB: %.2f", lowestPriceInPast7Days)
	}

	if strategy == enums.StrategyLTP99 {
		return Round(ltp * 0.99)
	}

	switch strategy {
	case enums.StrategyLTP99:
		return Round(ltp * 0.99)
	case enums.StrategyLTP98:
		return Round(ltp * 0.98)
	case enums.StrategyLTP95:
		return Round(ltp * 0.95)
	case enums.StrategyLtpLowestIn7days5t5:
		return Round(ltp*0.5 + lowestPriceInPast7Days*0.5)
	case enums.StrategyLtpLowestIn7days2t8:
		return Round(ltp*0.2 + lowestPriceInPast7Days*0.8)
	}

	if strategy == enums.Stg0BtcLtp3low7 {
		return Round(ltp*0.3 + low*0.7)
	} else if strategy == enums.Stg1BtcLtp997 {
		return Round(ltp * 0.997)
	} else if strategy == enums.Stg2BtcLtp98 {
		return Round(ltp * 0.98)
	} else if strategy == enums.Stg3BtcLtp90 {
		return Round(ltp * 0.90)
	} else if strategy == enums.Stg10EthLtp995 {
		return Round(ltp * 0.995)
	} else if strategy == enums.Stg11EthLtp98 {
		return Round(ltp * 0.98)
	} else if strategy == enums.Stg12EthLtp97 {
		return Round(ltp * 0.97)
	} else if strategy == enums.Stg13EthLtp3low7 {
		return Round(ltp*0.3 + low*0.7)
	} else if strategy == enums.Stg14EthLtp90 {
		return Round(ltp * 0.90)
	} else {
		return Round(ltp * 0.95)
	}
}
