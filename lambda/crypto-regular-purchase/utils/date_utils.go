package utils

import (
	"fmt"
	"time"
)

func IsTodayHoliday() bool {
	//https://holidays-jp.github.io/api/v1/date.json
	holidays := []string{
		"2025-07-21", // 海の日
		"2025-08-11", // 山の日
		"2025-09-15", // 敬老の日
		"2025-09-23", // 秋分の日
		"2025-10-13", // スポーツの日
		"2025-11-03", // 文化の日
		"2025-11-23", // 勤労感謝の日
		"2025-11-24", // 勤労感謝の日 振替休日
		"2025-12-30", // 年末
		"2025-12-31", // 年末
		"2026-01-01", // 元日
		"2026-01-02", // 元日
		"2026-01-03", // 元日
		"2026-01-12", // 成人の日
		"2026-02-11", // 建国記念の日
		"2026-02-23", // 天皇誕生日
		"2026-03-20", // 春分の日
		"2026-04-29", // 昭和の日
		"2026-05-03", // 憲法記念日
		"2026-05-04", // みどりの日
		"2026-05-05", // こどもの日
		"2026-05-06", // こどもの日 振替休日
		"2026-07-20", // 海の日
		"2026-08-11", // 山の日
		"2026-09-21", // 敬老の日
		"2026-09-22", // 国民の休日
		"2026-09-23", // 秋分の日
		"2026-10-12", // スポーツの日
		"2026-11-03", // 文化の日
		"2026-11-23", // 勤労感謝の日
		"2026-12-30", // 年末
		"2026-12-31", // 年末
		"2027-01-01", // 元日
		"2027-01-02", // 元日
		"2027-01-03", // 元日
	}

	jst, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		panic(err)
	}

	now := time.Now().In(jst)
	const dateFormat = "2006-01-02"
	today := now.Format(dateFormat)
	weekday := now.Weekday()
	fmt.Printf("Today is %s %s\n", today, weekday)

	if weekday == time.Saturday || weekday == time.Sunday {
		return true
	}

	for _, d := range holidays {
		if d == today {
			fmt.Println("Today is not a working day!!")
			return true
		}
	}

	fmt.Println("Today is a working day!!")
	return false
}
