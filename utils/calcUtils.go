package utils

import (
	"log"
	"math"
)

func Round(f float64) float64 {
	return math.Floor(f + .5)
}

func CalculateBuyPrice(ltp, low float64, strategy int) float64 {
	log.Printf("LTP:%10.2f  BestBid:%10.2f ", ltp, low)

	if strategy == 0 { // BTC_JPY
		return Round(ltp*0.3 + low*0.7)
	} else if strategy == 1 { // BTC_JPY
		return Round(ltp * 0.997)
	} else if strategy == 2 { // BTC_JPY
		return Round(ltp * 0.98)
	} else if strategy == -1 { // for test
		return Round(ltp * 0.8)
	} else if strategy == 10 { // ETH_JPY
		return Round(ltp * 0.995)
	} else if strategy == 11 { // ETH_JPY
		return Round(ltp * 0.98)
	} else if strategy == 12 { // ETH_JPY
		return Round(ltp * 0.97)
	} else {
		return Round(ltp*0.3 + low*0.7)
	}
}
