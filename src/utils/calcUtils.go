package utils

import (
	"math"
)

func Round(f float64) float64{
	return math.Floor(f + .5) 
}
