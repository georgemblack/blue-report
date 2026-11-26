package state

import (
	"context"
	"fmt"
	"time"

	"github.com/georgemblack/blue-report/backend/internal/event"
	"github.com/georgemblack/blue-report/backend/internal/eventlog"
	"github.com/georgemblack/blue-report/backend/internal/ranking"
	"github.com/georgemblack/blue-report/backend/internal/store"
)

// RebuildStats describes a startup rebuild, for logging.
type RebuildStats struct {
	Files    int
	Events   int // rows read
	Restored int // rows applied (the rest were too old or ignored)
	Took     time.Duration
}

// Rebuild restores the actor's state from the last week of stored events,
// ending at now (the newest stored event time). Call it before Run.
func (a *Actor) Rebuild(ctx context.Context, s store.Store, now time.Time, workers int) (RebuildStats, error) {
	start := time.Now()
	keys, err := eventlog.FilesSince(ctx, s, now.Add(-ranking.Week), now)
	if err != nil {
		return RebuildStats{}, err
	}
	stats := RebuildStats{Files: len(keys)}
	err = eventlog.ReadFiles(ctx, s, keys, workers, func(_ string, events []event.Event) error {
		stats.Events += len(events)
		stats.Restored += a.proc.Restore(events, now)
		return nil
	})
	if err != nil {
		return stats, fmt.Errorf("state: rebuilding: %w", err)
	}
	stats.Took = time.Since(start)
	return stats, nil
}
