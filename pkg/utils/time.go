package utils

import (
	"fmt"
	"time"
)

func EndOfDay(date time.Time, timezone string) (time.Time, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timezone %q: %w", timezone, err)
	}

	endOfDay := time.Date(
		date.Year(),
		date.Month(),
		date.Day(),
		23, 59, 59, 0,
		location,
	)
	return endOfDay, nil
}
