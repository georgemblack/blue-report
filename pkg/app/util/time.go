package util

import "time"

func ToEastern(t time.Time) time.Time {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		return t
	}
	return t.In(location)
}
