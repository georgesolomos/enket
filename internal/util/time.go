package util

import "time"

func InDateRange(start, end, date time.Time) bool {
	if start.Equal(end) {
		return date.Equal(start)
	}
	if start.After(end) {
		return !start.After(date) || !end.Before(date)
	}
	return date.After(start) && date.Before(end)
}

func IsMidnight(t time.Time) bool {
	return t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0
}

func DaysInMonth(t time.Time) int {
	if t.Month() == time.February && isLeap(t.Year()) {
		return 29
	}
	return daysInMonth[t.Month()]
}

func isLeap(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

var daysInMonth = []int{
	0,
	31, // Jan
	28, // Feb
	31, // Mar
	30, // Apr
	31, // May
	30, // Jun
	31, // Jul
	31, // Aug
	30, // Sep
	31, // Oct
	30, // Nov
	31, // Dec
}
