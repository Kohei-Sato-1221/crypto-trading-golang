package utils

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

var (
	Layout = "2006-01-02 15:04:05"

	// OkexCancelCriteria / OkjCancelCriteria は各取引所のタスクが未約定注文を
	// キャンセルするまでの経過日数（AddDate に渡す負値）。
	// Bitflyer 向けの BfCancelCriteria は撤去済み。cancelBuyOrderJob は固定日数ではなく
	// config.Config.BFBuyOrderCancelDays（config.ini の buy_order_cancel_days）を使う。
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

func ToP[T any](value T) *T {
	return &value
}

func RoundDecimal(num float64) float64 {
	return math.Round(num*100) / 100
}

// BitflyerTimeLayout はBitflyerが返す時刻文字列のフォーマット。
// タイムゾーンサフィックスが無く、値はUTC・秒精度（実APIレスポンスで確認済み）。
//
//	例: "expire_date":"2026-10-05T22:41:05" / "child_order_date":"2026-09-05T22:41:05"
const BitflyerTimeLayout = "2006-01-02T15:04:05"

/*
ParseBitflyerTime はBitflyerが返す時刻文字列をUTCのtime.Timeにパースする。

対応形式:
  - "2006-01-02T15:04:05"          （実測形式。サフィックス無し = UTC）
  - "2006-01-02T15:04:05.999999"   （小数秒が付与されるケース）
  - RFC3339                        （将来サフィックスが付いた場合の保険）

タイムゾーンサフィックスが無い値はUTCとして解釈する。DBへはUTCで保存すること。
*/
func ParseBitflyerTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("empty bitflyer time value")
	}
	if t, err := time.ParseInLocation(BitflyerTimeLayout, value, time.UTC); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.ParseInLocation(BitflyerTimeLayout+".999999999", value, time.UTC); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("failed to parse bitflyer time: %s", value)
}
