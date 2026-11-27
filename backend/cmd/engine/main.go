// Command engine reads Bluesky's Jetstream, keeps the rankings in memory,
// saves every counted event as Parquet files, and publishes top-links.json
// with each link's title, thumbnail and recommended posts, along with a feed
// of the day's top links. Popular links are
// checked for redirects, so short links count toward the article they lead
// to.
//
// Configuration comes from environment variables; see internal/config. With
// none set, it runs locally, using folders under local-data/ in place of S3
// and R2.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/georgemblack/blue-report/backend/internal/config"
	"github.com/georgemblack/blue-report/backend/internal/event"
	"github.com/georgemblack/blue-report/backend/internal/eventlog"
	"github.com/georgemblack/blue-report/backend/internal/feed"
	"github.com/georgemblack/blue-report/backend/internal/hydrate"
	"github.com/georgemblack/blue-report/backend/internal/publish"
	"github.com/georgemblack/blue-report/backend/internal/resolve"
	"github.com/georgemblack/blue-report/backend/internal/state"
	"github.com/georgemblack/blue-report/backend/internal/stream"
)

const (
	// How far back to look for the newest saved file when starting up.
	resumeLookback = 7 * 24 * time.Hour

	// Room for a few seconds of events while the actor is busy (for example
	// computing the rankings).
	eventsBuffer = 100_000

	// How often to publish top-links.json.
	publishInterval = 5 * time.Minute

	// How many event files to download and decode at once during the
	// startup rebuild.
	rebuildWorkers = 8

	// How many batches can wait for the writer before the actor has to wait.
	batchesBuffer = 4

	// ECS gives the container 120 seconds after SIGTERM. Stop retrying
	// uploads a little before that so we can log what wasn't saved.
	shutdownDeadline = 100 * time.Second

	// Where published files can be read, for the feeds' own addresses.
	publicURL = "https://data.theblue.report/"
)

// commit is the git commit the binary was built from, set at build time
// (see the Dockerfile). "dev" means a local build.
var commit = "dev"

func main() {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	logger := newLogger(cfg.LogFormat)
	if err := run(cfg, logger); err != nil {
		logger.Error("engine failed", "error", err)
		os.Exit(1)
	}
	logger.Info("engine stopped")
}

func run(cfg config.Config, logger *slog.Logger) error {
	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	awsCfg, err := awsConfig(ctx)
	if err != nil {
		return err
	}
	events, eventsWhere := eventStore(awsCfg, cfg)
	published, thumbnails, publishWhere, err := publishStores(ctx, cfg)
	if err != nil {
		return err
	}
	titles, titlesWhere := titleStore(awsCfg, cfg)
	redirects, redirectsWhere := redirectStore(awsCfg, cfg)
	feedEntries, feedWhere := feedStore(awsCfg, cfg)
	browser, browserOn := browser(cfg)
	logger.Info("starting engine", "commit", commit, "events", eventsWhere, "publish", publishWhere,
		"titles", titlesWhere, "redirects", redirectsWhere, "feed", feedWhere, "browser_run", browserOn)

	cursor, err := eventlog.ResumePoint(ctx, events, time.Now(), resumeLookback)
	if err != nil {
		return err
	}
	if cursor == 0 {
		logger.Info("no saved events found; starting from now")
	} else {
		resumeAt := time.UnixMicro(cursor)
		logger.Info("resuming from newest saved event", "resume_at", resumeAt.UTC(), "behind", time.Since(resumeAt).Round(time.Second).String())
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

	// Check popular links for redirects. The resolver stops with the
	// reader; any links still waiting are checked again after a restart.
	actor := state.NewActor(state.DefaultConfig(), logger)
	go resolve.New(redirects, actor, logger).Run(ctx, actor.Popular())

	// Rebuild the rankings and post cache from the last week of saved
	// events before reading anything new. Links that are already popular
	// are sent to the resolver again along the way.
	if cursor > 0 {
		stats, err := actor.Rebuild(ctx, events, time.UnixMicro(cursor), rebuildWorkers)
		if err != nil {
			return err
		}
		logger.Info("rebuilt state from saved events", "files", stats.Files, "events", stats.Events, "restored", stats.Restored, "took", stats.Took.Round(time.Millisecond).String())
	}
	go actor.Run(messages, batches)

	// Publish once the state is rebuilt, whether or not the engine has
	// caught up with Jetstream yet.
	publishCtx, stopPublishing := context.WithCancel(ctx)
	defer stopPublishing()
	publisherDone := make(chan struct{})
	go func() {
		hydrator := hydrate.New(titles, thumbnails, browser, logger)
		topFeed := feed.New(feedEntries, published, publicURL+cfg.PublishPrefix, logger)
		publish.NewPublisher(actor, hydrator, topFeed, published, publishInterval, logger).Run(publishCtx)
		close(publisherDone)
	}()

	// Read until a shutdown signal (or a fatal stream error).
	readErr := stream.NewReader(logger).Run(ctx, cursor, messages)
	if errors.Is(readErr, context.Canceled) {
		readErr = nil
	}
	stopSignals() // a second Ctrl-C now exits immediately
	logger.Info("shutting down: saving remaining events")
	// Stop publishing whatever stopped the reader (a signal or a fatal error).
	stopPublishing()
	<-publisherDone

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

func newLogger(format string) *slog.Logger {
	if format == "text" {
		return slog.New(slog.NewTextHandler(os.Stdout, nil))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}
