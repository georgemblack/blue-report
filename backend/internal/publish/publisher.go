package publish

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/georgemblack/blue-report/backend/internal/state"
	"github.com/georgemblack/blue-report/backend/internal/store"
)

// TopLinksKey is where top-links.json is published.
const TopLinksKey = "data/top-links.json"

// Ranker gives the current rankings. The state actor implements it.
type Ranker interface {
	Top(ctx context.Context) (state.TopResult, error)
}

// Publisher uploads top-links.json on a timer.
type Publisher struct {
	ranker   Ranker
	store    store.Store
	logger   *slog.Logger
	interval time.Duration
	now      func() time.Time // for tests
}

func NewPublisher(r Ranker, s store.Store, interval time.Duration, logger *slog.Logger) *Publisher {
	return &Publisher{ranker: r, store: s, logger: logger, interval: interval, now: time.Now}
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

// Publish builds top-links.json from the current rankings and uploads it.
func (p *Publisher) Publish(ctx context.Context) error {
	start := p.now()
	res, err := p.ranker.Top(ctx)
	if err != nil {
		return fmt.Errorf("publish: getting rankings: %w", err)
	}
	data, err := json.Marshal(BuildSnapshot(res, start))
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
		"redirect_candidates", len(res.Candidates), "bytes", len(data),
		"took_ms", p.now().Sub(start).Milliseconds())
	return nil
}
