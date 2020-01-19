package utils

import (
	"math"
)

func Round(f float64) float64{
	return math.Floor(f + .5) 
}

func CalculateBuyPrice(ltp, low float64, strategy int)  float64 {
	if strategy == 0 {
		return Round(ltp * 0.3  + low * 0.7)
	}else if strategy == 1 {
		return Round(ltp * 0.5  + low * 0.5)		
	}else if strategy == -1 { // for test
		return Round(ltp * 0.8)
	}else {
		return Round(ltp * 0.3  + low * 0.7)
	}
}