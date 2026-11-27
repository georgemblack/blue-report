// Package resolve finds where popular links redirect to, so a short link
// (e.g. nyti.ms/abc) and the article it points to are ranked as one link.
//
// What it finds is saved for 10 days, so after a restart, links it has
// already checked are answered from the saved copy instead of followed
// again.
package resolve

import (
	"context"
	"log/slog"
	"time"

	"github.com/georgemblack/blue-report/backend/internal/urltools"
)

// How long a saved answer is used before the link is followed again.
const keepFor = 10 * 24 * time.Hour

// Merger is told when a link redirects (the state actor).
type Merger interface {
	Merge(ctx context.Context, source, destination string) error
}

type Resolver struct {
	saved  Store
	merger Merger
	logger *slog.Logger

	// Follows a link's redirects and returns where they end, or "" if it
	// doesn't redirect. Replaced in tests.
	follow func(url string) string
	now    func() time.Time
}

func New(saved Store, merger Merger, logger *slog.Logger) *Resolver {
	return &Resolver{saved: saved, merger: merger, logger: logger, follow: urltools.FindRedirect, now: time.Now}
}

// Run checks each link it receives, one at a time, until ctx is cancelled.
func (r *Resolver) Run(ctx context.Context, links <-chan string) {
	for {
		select {
		case url := <-links:
			r.resolve(ctx, url)
		case <-ctx.Done():
			return
		}
	}
}

// resolve finds where a link redirects to, using the saved answer if there
// is one, and tells the merger if it's somewhere else.
func (r *Resolver) resolve(ctx context.Context, url string) {
	// Apple News pages redirect with JavaScript, not normal redirects, so
	// they're skipped for now. Browser Run could handle them later.
	if urltools.IsAppleNewsURL(url) {
		return
	}

	destination, found, err := r.saved.Get(ctx, url, r.now())
	if err != nil {
		r.logger.Warn("couldn't get saved redirect", "url", url, "error", err)
	}
	if !found {
		destination = r.find(url)
		if err := r.saved.Put(ctx, url, destination, r.now().Add(keepFor)); err != nil {
			r.logger.Warn("couldn't save redirect", "url", url, "error", err)
		}
	}
	if destination == "" {
		return
	}
	if err := r.merger.Merge(ctx, url, destination); err != nil {
		r.logger.Warn("couldn't merge redirect", "url", url, "destination", destination, "error", err)
		return
	}
	r.logger.Info("merged redirect", "url", url, "destination", destination, "saved", found)
}

// find follows a link's redirects. It returns the cleaned destination, or
// "" if the link doesn't redirect anywhere else.
func (r *Resolver) find(url string) string {
	redirect := r.follow(url)
	if redirect == "" {
		return ""
	}
	// Tracking parameters and the like may have been added along the way.
	destination := urltools.Clean(redirect)
	if destination == url {
		return ""
	}
	return destination
}
