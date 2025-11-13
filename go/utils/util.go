package utils

import (
	"math"
	"strconv"
)

var (
	Layout             = "2006-01-02 15:04:05"
	BfCancelCriteria   = -3
	OkexCancelCriteria = -3
	OkjCancelCriteria  = 0
)

func STf(str string) float64 {
	f64, error := strconv.ParseFloat(str, 64)
	if error != nil {
		return 0.00
	}
	return f64
}

func FTs(f64 float64) string {
	str := strconv.FormatFloat(f64, 'f', 3, 64)
	return str
}

func RoundDecimal(num float64) float64 {
	return math.Round(num*100) / 100
}
