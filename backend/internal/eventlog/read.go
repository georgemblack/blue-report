package eventlog

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/georgemblack/blue-report/backend/internal/event"
	"github.com/georgemblack/blue-report/backend/internal/store"
)

// FilesSince lists the event files whose newest event is at or after since,
// up to and including the day of until. It checks one day's folder at a
// time.
func FilesSince(ctx context.Context, s store.Store, since, until time.Time) ([]string, error) {
	sinceUS := since.UnixMicro()
	var keys []string
	last := until.UTC().Truncate(24 * time.Hour)
	for day := since.UTC().Truncate(24 * time.Hour); !day.After(last); day = day.AddDate(0, 0, 1) {
		dayKeys, err := s.List(ctx, fmt.Sprintf("%sdate=%s/", Prefix, day.Format(time.DateOnly)))
		if err != nil {
			return nil, fmt.Errorf("eventlog: listing files: %w", err)
		}
		for _, k := range dayKeys {
			if t, ok := timeFromKey(k); ok && t >= sinceUS {
				keys = append(keys, k)
			}
		}
	}
	return keys, nil
}

// ReadFiles downloads and decodes the given files, several at a time, and
// calls fn with each file's events. fn is always called from one goroutine
// at a time, but files arrive in no particular order. Only a few files are
// held in memory at once.
func ReadFiles(ctx context.Context, s store.Store, keys []string, workers int, fn func(key string, events []event.Event) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type result struct {
		key    string
		events []event.Event
		err    error
	}
	jobs := make(chan string)
	results := make(chan result, workers)

	var wg sync.WaitGroup
	for range max(workers, 1) {
		wg.Go(func() {
			for key := range jobs {
				data, err := s.Get(ctx, key)
				var events []event.Event
				if err == nil {
					events, err = Decode(data)
				}
				select {
				case results <- result{key: key, events: events, err: err}:
				case <-ctx.Done():
					return
				}
			}
		})
	}
	go func() {
		defer close(jobs)
		for _, k := range keys {
			select {
			case jobs <- k:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		if r.err != nil {
			return fmt.Errorf("eventlog: reading %s: %w", r.key, r.err)
		}
		if err := fn(r.key, r.events); err != nil {
			return err
		}
	}
	return ctx.Err()
}
