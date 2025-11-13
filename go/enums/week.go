package enums

import "time"

const (
	WeekdaySunday    string = "Sunday"
	WeekdayTuesday   string = "Tuesday"
	WeekdayWednesday string = "Wednesday"
	WeekdayThursday  string = "Thursday"
	WeekdayFriday    string = "Friday"
	WeekdaySaturday  string = "Saturday"
	WeekdayMonday    string = "Monday"
)

func getTodayWeekday() string {
	now := time.Now()
	weekday := now.Weekday()
	return weekday.String()
}

func IsTodayWeekday(weekday string) bool {
	return weekday == getTodayWeekday()
}
