package utils

import (
	"fmt"
	"time"
)

const (
	YYYY_MM_DD = "2006-01-02"
	MMDDYYYY   = "01022006"
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
