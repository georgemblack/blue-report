package publish

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/georgemblack/blue-report/backend/internal/feed"
	"github.com/georgemblack/blue-report/backend/internal/hydrate"
	"github.com/georgemblack/blue-report/backend/internal/ranking"
	"github.com/georgemblack/blue-report/backend/internal/state"
	"github.com/georgemblack/blue-report/backend/internal/store"
)

// TopLinksKey is where top-links.json is published.
const TopLinksKey = "data/top-links.json"

// Ranker gives the current rankings. The state actor implements it.
type Ranker interface {
	Top(ctx context.Context) (state.TopResult, error)
}

// Hydrator fills in titles, thumbnails and recommended posts. The hydrate
// package implements it.
type Hydrator interface {
	Hydrate(ctx context.Context, entries []ranking.Entry) map[string]hydrate.Info
}

// Feed keeps the feed of the day's top links. The feed package implements
// it.
type Feed interface {
	Update(ctx context.Context, top feed.Entry, now time.Time) error
}

// Publisher uploads top-links.json on a timer, and keeps the feed up to
// date.
type Publisher struct {
	ranker   Ranker
	hydrator Hydrator // nil to publish without titles and the rest
	feed     Feed     // nil for no feed
	store    store.Store
	logger   *slog.Logger
	interval time.Duration
	now      func() time.Time // for tests
}

func NewPublisher(r Ranker, h Hydrator, f Feed, s store.Store, interval time.Duration, logger *slog.Logger) *Publisher {
	return &Publisher{ranker: r, hydrator: h, feed: f, store: s, logger: logger, interval: interval, now: time.Now}
}

// Run publishes straight away, then every interval, until ctx is done. A
// failed publish is logged and tried again at the next interval.
func (p *Publisher) Run(ctx context.Context) {
	tick := time.NewTicker(p.interval)
	defer tick.Stop()
	for {
		if err := p.Publish(ctx); err != nil && ctx.Err() == nil {
			p.logger.Warn("couldn't publish top links", "error", err)
		}
		select {
		case <-tick.C:
		case <-ctx.Done():
			return
		}
	}
}

// Publish builds top-links.json from the current rankings, fills in each
// link's title, thumbnail and recommended posts, and uploads it. Then it
// offers the day's top link to the feed.
func (p *Publisher) Publish(ctx context.Context) error {
	start := p.now()
	res, err := p.ranker.Top(ctx)
	if err != nil {
		return fmt.Errorf("publish: getting rankings: %w", err)
	}
	var info map[string]hydrate.Info
	if p.hydrator != nil {
		info = p.hydrator.Hydrate(ctx, allEntries(res.Ranking))
	}
	snapshot := BuildSnapshot(res, info, start)
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("publish: encoding top links: %w", err)
	}
	meta := store.Meta{
		ContentType: "application/json",
		// Match the publish interval, so browsers and Cloudflare don't hold
		// on to a copy longer than it's current.
		CacheControl: fmt.Sprintf("public, max-age=%d", int(p.interval.Seconds())),
	}
	if err := p.store.Put(ctx, TopLinksKey, data, meta); err != nil {
		return fmt.Errorf("publish: uploading top links: %w", err)
	}
	p.logger.Info("published top links",
		"hour", len(res.Ranking.Hour), "day", len(res.Ranking.Day), "week", len(res.Ranking.Week),
		"bytes", len(data),
		"took_ms", p.now().Sub(start).Milliseconds())

	// A feed problem doesn't count as a failed publish; it's tried again
	// next time.
	if p.feed != nil && len(snapshot.TopDay) > 0 {
		if err := p.feed.Update(ctx, feedEntry(snapshot.TopDay[0]), start); err != nil {
			p.logger.Warn("couldn't update feed", "error", err)
		}
	}
	return nil
}

// feedEntry turns a published link into a feed entry. A link without a
// real title gets an empty one, so the feed waits for its title.
func feedEntry(l Link) feed.Entry {
	e := feed.Entry{URL: l.URL, Title: l.Title, Posts: []feed.Post{}}
	if e.Title == FallbackTitle {
		e.Title = ""
	}
	for _, p := range l.RecommendedPosts {
		e.Posts = append(e.Posts, feed.Post{Rank: p.Rank, AtURI: p.AtURI, Username: p.Username, Handle: p.Handle, Text: p.Text})
	}
	return e
}
