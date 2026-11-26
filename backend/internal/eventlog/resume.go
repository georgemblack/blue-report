package eventlog

import (
	"context"
	"fmt"
	"time"

	"github.com/georgemblack/blue-report/backend/internal/store"
)

// ResumePoint returns the newest event time stored, in microseconds since
// the Unix epoch, which is where to resume reading Jetstream. It returns 0
// if there are no files from the last lookback period (for example, on the
// first run), meaning start from now.
//
// It checks one day's folder at a time, starting today, so it never has to
// list the whole bucket.
func ResumePoint(ctx context.Context, s store.Store, now time.Time, lookback time.Duration) (int64, error) {
	day := now.UTC().Truncate(24 * time.Hour)
	oldest := now.UTC().Add(-lookback).Truncate(24 * time.Hour)
	for ; !day.Before(oldest); day = day.AddDate(0, 0, -1) {
		keys, err := s.List(ctx, fmt.Sprintf("%sdate=%s/", Prefix, day.Format(time.DateOnly)))
		if err != nil {
			return 0, fmt.Errorf("eventlog: finding resume point: %w", err)
		}
		// Keys are sorted, and both the hour folders and the file names
		// sort correctly as text, so the newest file is listed last.
		for i := len(keys) - 1; i >= 0; i-- {
			if t, ok := timeFromKey(keys[i]); ok {
				return t, nil
			}
		}
	}
	return 0, nil
}
