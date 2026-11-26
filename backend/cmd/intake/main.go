// Command intake reads Bluesky's Jetstream, keeps the rankings in memory and
// saves every counted event as Parquet files.
//
// Configuration (environment variables):
//
//	BLUE_REPORT_DATA_DIR  folder used as the event store (default "local-data")
//	LOG_FORMAT            "json" (default) or "text"
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/georgemblack/blue-report/backend/internal/event"
	"github.com/georgemblack/blue-report/backend/internal/eventlog"
	"github.com/georgemblack/blue-report/backend/internal/state"
	"github.com/georgemblack/blue-report/backend/internal/store"
	"github.com/georgemblack/blue-report/backend/internal/stream"
)

const (
	// How far back to look for the newest saved file when starting up.
	resumeLookback = 7 * 24 * time.Hour

	// Room for a few seconds of events while the actor is busy (for example
	// computing the rankings).
	eventsBuffer = 100_000

	// How many batches can wait for the writer before the actor has to wait.
	batchesBuffer = 4

	// ECS gives the container 120 seconds after SIGTERM. Stop retrying
	// uploads a little before that so we can log what wasn't saved.
	shutdownDeadline = 100 * time.Second
)

func main() {
	logger := newLogger()
	if err := run(logger); err != nil {
		logger.Error("intake failed", "error", err)
		os.Exit(1)
	}
	logger.Info("intake stopped")
}

func run(logger *slog.Logger) error {
	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	dir := envOr("BLUE_REPORT_DATA_DIR", "local-data")
	events := store.NewLocal(dir)

	cursor, err := eventlog.ResumePoint(ctx, events, time.Now(), resumeLookback)
	if err != nil {
		return err
	}
	if cursor == 0 {
		logger.Info("no saved events found; starting from now", "data_dir", dir)
	} else {
		resumeAt := time.UnixMicro(cursor)
		logger.Info("resuming from newest saved event", "data_dir", dir, "resume_at", resumeAt.UTC(), "behind", time.Since(resumeAt).Round(time.Second).String())
	}

	messages := make(chan stream.Message, eventsBuffer)
	batches := make(chan []event.Event, batchesBuffer)

	// The writer gets its own context: it keeps saving after a shutdown
	// signal, until everything is saved or the deadline passes.
	writeCtx, cancelWrites := context.WithCancel(context.Background())
	defer cancelWrites()
	writer := eventlog.NewWriter(events, logger)
	writerDone := make(chan error, 1)
	go func() { writerDone <- writer.Run(writeCtx, batches) }()

	actor := state.NewActor(state.DefaultConfig(), logger)
	go actor.Run(messages, batches)

	// Read until a shutdown signal (or a fatal stream error).
	readErr := stream.NewReader(logger).Run(ctx, cursor, messages)
	if errors.Is(readErr, context.Canceled) {
		readErr = nil
	}
	stopSignals() // a second Ctrl-C now exits immediately
	logger.Info("shutting down: saving remaining events")

	// Shutdown order: the actor handles what's left and hands its last
	// batch to the writer, then closes batches; the writer saves everything
	// and returns.
	close(messages)
	deadline := time.AfterFunc(shutdownDeadline, cancelWrites)
	defer deadline.Stop()
	if err := <-writerDone; err != nil {
		return errors.Join(readErr, err)
	}
	return readErr
}

func newLogger() *slog.Logger {
	if os.Getenv("LOG_FORMAT") == "text" {
		return slog.New(slog.NewTextHandler(os.Stdout, nil))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
