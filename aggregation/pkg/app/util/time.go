package util

import (
	"log/slog"
	"time"
)

func ToEastern(t time.Time) time.Time {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		slog.Error(WrapErr("failed to load time location when converting time to eastern", err).Error())
		return t
	}
	return t.In(location)
}
