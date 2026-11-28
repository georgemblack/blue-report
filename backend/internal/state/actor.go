package state

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"time"

	"github.com/georgemblack/blue-report/backend/internal/event"
	"github.com/georgemblack/blue-report/backend/internal/ranking"
	"github.com/georgemblack/blue-report/backend/internal/stream"
)

// Config controls the actor's timers and limits.
type Config struct {
	WriteInterval time.Duration // how often to hand the buffer to the writer
	MaxBuffer     int           // hand it over early once it holds this many events
	SweepInterval time.Duration // how often to forget old posts and interactions
	StatsInterval time.Duration // how often to log a stats line

	TopN             int // links per list in a Top result
	RedirectMinUsers int // user entries a link needs before it's checked for redirects
	MaxPopular       int // popular links that can wait for the resolver
}

func DefaultConfig() Config {
	return Config{
		WriteInterval:    30 * time.Minute,
		MaxBuffer:        100_000,
		SweepInterval:    10 * time.Minute,
		StatsInterval:    time.Minute,
		TopN:             10,
		RedirectMinUsers: 1000,
		MaxPopular:       10_000,
	}
}

// ErrStopped is returned by Top and Merge once the actor has shut down.
var ErrStopped = errors.New("state: actor has stopped")

// TopResult is a snapshot of the rankings, copied out of the actor's state
// so it can be used from any goroutine.
type TopResult struct {
	Now     time.Time      // newest event time: "now" for the ranking windows
	Ranking ranking.Result // top links for the hour, day and week
}

type topRequest struct {
	reply chan TopResult // buffered, so the actor never waits on it
}

type mergeRequest struct {
	source, destination string
}

// Actor is the only goroutine that touches the state: the post cache,
// ranking, translations and event buffer. Other goroutines
// send it messages. It does no network or disk work, so nothing slow can
// hold up event processing.
type Actor struct {
	cfg    Config
	logger *slog.Logger
	proc   *Processor

	topRequests chan topRequest
	merges      chan mergeRequest
	stopped     chan struct{} // closed when Run returns
	popular     chan string   // links that just became popular, for the resolver

	// For the stats line.
	lastStats   Stats
	lastStatsAt time.Time
	lastTopTook time.Duration
}

func NewActor(cfg Config, logger *slog.Logger) *Actor {
	a := &Actor{
		cfg:         cfg,
		logger:      logger,
		proc:        NewProcessor(),
		topRequests: make(chan topRequest),
		merges:      make(chan mergeRequest),
		stopped:     make(chan struct{}),
		popular:     make(chan string, cfg.MaxPopular),
	}
	a.proc.popularMinUsers = cfg.RedirectMinUsers
	a.proc.popular = a.sendPopular
	return a
}

// Popular returns the links to check for redirects: each link is sent when
// it reaches RedirectMinUsers user entries. During the startup rebuild,
// every link that's already popular is sent again.
func (a *Actor) Popular() <-chan string {
	return a.popular
}

// sendPopular hands a link to the resolver without waiting, so the actor
// never stops for it. If the resolver is far behind, the link is skipped.
func (a *Actor) sendPopular(url string) {
	select {
	case a.popular <- url:
	default:
		a.logger.Warn("too many links waiting for the resolver; skipping", "url", url)
	}
}

// Run handles messages until the events channel is closed. Then it hands
// the last buffer to the writer, closes batches and returns.
//
// Sending to batches waits if the writer is behind. That stops the actor
// reading events, and so stops the reader reading Jetstream, rather than
// letting memory grow.
func (a *Actor) Run(events <-chan stream.Message, batches chan<- []event.Event) {
	defer close(a.stopped)
	defer close(batches)

	writeTick := time.NewTicker(a.cfg.WriteInterval)
	sweepTick := time.NewTicker(a.cfg.SweepInterval)
	statsTick := time.NewTicker(a.cfg.StatsInterval)
	defer writeTick.Stop()
	defer sweepTick.Stop()
	defer statsTick.Stop()
	a.lastStatsAt = time.Now()

	for {
		select {
		case msg, ok := <-events:
			if !ok {
				// The reader has stopped and everything it sent has been
				// handled. Save what's left and finish.
				a.flush(batches)
				a.logStats()
				return
			}
			a.proc.Handle(msg)
			if a.proc.BufferLen() >= a.cfg.MaxBuffer {
				a.flush(batches)
			}

		case req := <-a.topRequests:
			req.reply <- a.top()

		case m := <-a.merges:
			a.proc.Merge(m.source, m.destination)

		case <-writeTick.C:
			a.flush(batches)

		case <-sweepTick.C:
			a.sweep()

		case <-statsTick.C:
			a.logStats()
		}
	}
}

// Top asks the actor for the current rankings and waits for the answer.
// Safe to call from any goroutine.
func (a *Actor) Top(ctx context.Context) (TopResult, error) {
	req := topRequest{reply: make(chan TopResult, 1)}
	select {
	case a.topRequests <- req:
	case <-a.stopped:
		return TopResult{}, ErrStopped
	case <-ctx.Done():
		return TopResult{}, ctx.Err()
	}
	// Once the actor has the request, it always answers straight away.
	return <-req.reply, nil
}

// Merge tells the actor that source redirects to destination. Safe to call
// from any goroutine.
func (a *Actor) Merge(ctx context.Context, source, destination string) error {
	select {
	case a.merges <- mergeRequest{source: source, destination: destination}:
		return nil
	case <-a.stopped:
		return ErrStopped
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *Actor) flush(batches chan<- []event.Event) {
	if a.proc.BufferLen() == 0 {
		return
	}
	batches <- a.proc.TakeBuffer()
}

func (a *Actor) top() TopResult {
	start := time.Now()
	now := a.proc.Newest()
	res := TopResult{Now: now, Ranking: a.proc.ranking.Top(now, a.cfg.TopN)}
	a.lastTopTook = time.Since(start)
	return res
}

// sweep forgets posts and interactions too old to matter, measured from
// the newest event time.
func (a *Actor) sweep() {
	now := a.proc.Newest()
	if now.IsZero() {
		return
	}
	start := time.Now()
	removedPosts := a.proc.posts.Sweep(now.Add(-PostTTL))
	a.proc.ranking.Sweep(now.Add(-ranking.Week))
	a.logger.Info("swept state", "removed_posts", removedPosts, "took_ms", time.Since(start).Milliseconds())
}

func (a *Actor) logStats() {
	now := time.Now()
	s := a.proc.Stats()
	d := s.sub(a.lastStats)
	secs := now.Sub(a.lastStatsAt).Seconds()
	a.lastStats, a.lastStatsAt = s, now

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	r := a.proc.ranking.Stats()

	attrs := []any{
		"recorded_per_sec", perSec(d.recorded(), secs),
		"skipped_per_sec", perSec(d.skipped(), secs),
		"posts", d.Posts, "quotes", d.Quotes, "reposts", d.Reposts, "likes", d.Likes,
		"no_link", d.NoLink, "ignored", d.Ignored, "not_cached", d.NotCached,
		"post_cache", a.proc.posts.Len(),
		"links", r.Links, "user_entries", r.Users, "post_entries", r.Posts,
		"translations", a.proc.translations.Len(),
		"buffer", a.proc.BufferLen(),
		"top_ms", a.lastTopTook.Milliseconds(),
		"heap_mb", mem.HeapAlloc >> 20,
	}
	if newest := a.proc.Newest(); !newest.IsZero() {
		attrs = append(attrs, "lag_ms", now.Sub(newest).Milliseconds())
	}
	a.logger.Info("stats", attrs...)
}

func perSec(n int, secs float64) float64 {
	if secs <= 0 {
		return 0
	}
	return float64(int(float64(n)/secs*10)) / 10
}
