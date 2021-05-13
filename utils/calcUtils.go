package utils

import (
	"github.com/Kohei-Sato-1221/crypto-trading-golang/enums"
	"log"
	"math"
)

func Round(f float64) float64 {
	return math.Floor(f + .5)
}

func CalculateBuyPrice(ltp, low float64, strategy int) float64 {
	log.Printf("LTP:%10.2f  BestBid:%10.2f ", ltp, low)

	if strategy == enums.TEST_STG {
		return Round(ltp * 0.8)
	} else if strategy == enums.Stg0BtcLtp3low7 {
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
