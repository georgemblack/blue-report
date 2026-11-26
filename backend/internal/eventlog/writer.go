package eventlog

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/georgemblack/blue-report/backend/internal/event"
	"github.com/georgemblack/blue-report/backend/internal/store"
)

// Writer saves batches of events as files, one at a time and in order.
type Writer struct {
	store  store.Store
	logger *slog.Logger

	// How long to wait before retrying a failed upload. The wait doubles
	// after each failure, up to retryMax.
	retryMin, retryMax time.Duration

	lastWrite atomic.Int64 // when the last file was saved, in Unix nanoseconds
}

func NewWriter(s store.Store, logger *slog.Logger) *Writer {
	return &Writer{store: s, logger: logger, retryMin: time.Second, retryMax: time.Minute}
}

// Run saves each batch received from batches until the channel is closed.
//
// A failed upload is retried until it works. Batches are never skipped,
// because the newest file is where reading resumes after a restart. While
// Run retries, batches wait in the channel; once it's full, the state actor
// waits too, which stops reading from Jetstream instead of running out of
// memory.
//
// Cancelling ctx stops the retrying (for a hard deadline at shutdown); Run
// then returns an error saying how many events weren't saved.
func (w *Writer) Run(ctx context.Context, batches <-chan []event.Event) error {
	for batch := range batches {
		if len(batch) == 0 {
			continue
		}
		data, maxTimeUS, err := Encode(batch)
		if err != nil {
			// Retrying won't help with data that can't be encoded.
			w.logger.Error("couldn't encode events; skipping batch", "events", len(batch), "error", err)
			continue
		}
		key := Key(maxTimeUS)
		if err := w.put(ctx, key, data); err != nil {
			unsaved := len(batch)
			for b := range batches {
				unsaved += len(b)
			}
			return fmt.Errorf("eventlog: stopped before saving %d events: %w", unsaved, err)
		}
		w.lastWrite.Store(time.Now().UnixNano())
		w.logger.Info("saved events", "key", key, "events", len(batch), "bytes", len(data))
	}
	return nil
}

func (w *Writer) put(ctx context.Context, key string, data []byte) error {
	wait := w.retryMin
	for {
		err := w.store.Put(ctx, key, data, store.Meta{ContentType: "application/vnd.apache.parquet"})
		if err == nil {
			return nil
		}
		w.logger.Warn("couldn't save events; will retry", "key", key, "retry_in", wait, "error", err)
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
		wait = min(wait*2, w.retryMax)
	}
}

// LastWrite returns when the last file was saved, or the zero time if none
// has been. Safe to call from any goroutine.
func (w *Writer) LastWrite() time.Time {
	ns := w.lastWrite.Load()
	if ns == 0 {
		return time.Time{}
	}
	return time.Unix(0, ns)
}
