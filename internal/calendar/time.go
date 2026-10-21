package calendar

import (
	"fmt"
	"time"
)

func EndOfDay(date time.Time, timezone string) (time.Time, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timezone %q: %w", timezone, err)
	}

	date = date.In(location)

	endOfDay := time.Date(
		date.Year(),
		date.Month(),
		date.Day(),
		23, 59, 59, 0,
		location,
	)

	return endOfDay, nil
}

func DaysBetween(from, to time.Time) int {
	fromDay := utcDate(from.In(to.Location()))
	toDay := utcDate(to)
	return int(toDay.Sub(fromDay).Hours() / 24)
}

func utcDate(moment time.Time) time.Time {
	year, month, day := moment.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
