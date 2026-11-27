// Package hydrate fills in what the rankings don't know about a link: its
// title, a thumbnail, and a few posts worth reading about it.
//
// Titles and preview images come from CardyB, or from Browser Run when
// CardyB can't read a page. It shares v1's titles (DynamoDB) and thumbnails
// (R2), so links v1 has already seen need no new lookups.
package hydrate

import (
	"cmp"
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/georgemblack/blue-report/backend/internal/browserrun"
	"github.com/georgemblack/blue-report/backend/internal/ranking"
	"github.com/georgemblack/blue-report/backend/internal/store"
)

// Info is what the website shows for a link besides its counts.
type Info struct {
	Title        string // "" if none was found
	ThumbnailURL string // "" if none was found
	Posts        []Post // recommended posts, best first; never nil
}

type Post struct {
	AtURI    string
	Username string // display name
	Handle   string
	Text     string
}

// Rules for recommending a post, as in v1.
const (
	maxPosts     = 3  // per link
	minPostLikes = 50 // a post needs more likes than this
)

// Browser loads pages in a real browser (Cloudflare Browser Run). The
// browserrun package implements it.
type Browser interface {
	Scrape(ctx context.Context, url string, selectors []string) (browserrun.Result, time.Duration, error)
}

// What Browser Run is asked to find on a page.
const (
	ogTitle   = `meta[property="og:title"]`
	ogImage   = `meta[property="og:image"]`
	pageTitle = "title"
)

// Hydrator looks up Info for the links being published. It's used by the
// publisher only, one call at a time.
type Hydrator struct {
	titles     Titles
	thumbnails store.Store // the R2 bucket, without the engine's prefix
	browser    Browser     // nil to use CardyB only
	logger     *slog.Logger
	client     *http.Client

	cardyBURL   string
	cardyBDelay time.Duration
	blueskyURL  string

	// Title and thumbnail for each link in the last publish, so they're only
	// looked up when a link first appears.
	pages map[string]page

	// Links in the last publish that Browser Run has already tried. Links
	// without a title are looked up on every publish, but Browser Run costs
	// money, so it only tries each link once while it's published.
	browsed map[string]bool
}

type page struct {
	title     string
	thumbnail string
}

// New returns a Hydrator. browser may be nil.
func New(titles Titles, thumbnails store.Store, browser Browser, logger *slog.Logger) *Hydrator {
	return &Hydrator{
		titles:     titles,
		thumbnails: thumbnails,
		browser:    browser,
		logger:     logger,
		// A timeout so one stuck request can't stop publishing for good.
		client:      &http.Client{Timeout: 30 * time.Second},
		cardyBURL:   "https://cardyb.bsky.app",
		cardyBDelay: time.Second,
		blueskyURL:  "https://public.api.bsky.app",
		pages:       map[string]page{},
		browsed:     map[string]bool{},
	}
}

// Hydrate returns Info for each of the given links, keyed by URL. Anything
// that can't be found is left empty; problems are logged, never returned.
func (h *Hydrator) Hydrate(ctx context.Context, entries []ranking.Entry) map[string]Info {
	start := time.Now()
	looked := 0
	pages := make(map[string]page, len(entries))
	remember := make(map[string]page, len(entries))
	for _, e := range entries {
		if _, done := pages[e.URL]; done {
			continue // a link can be in more than one list
		}
		p, ok := h.pages[e.URL]
		if !ok {
			p = h.lookup(ctx, e.URL)
			looked++
		}
		pages[e.URL] = p
		// Links without a title are looked up again next time.
		if p.title != "" {
			remember[e.URL] = p
		}
	}
	// Forget links that are no longer published.
	h.pages = remember
	for url := range h.browsed {
		if _, ok := pages[url]; !ok {
			delete(h.browsed, url)
		}
	}

	posts := h.recommend(ctx, entries)
	info := make(map[string]Info, len(pages))
	for url, p := range pages {
		info[url] = Info{Title: p.title, ThumbnailURL: p.thumbnail, Posts: posts[url]}
	}
	h.logger.Info("hydrated links", "links", len(pages), "new", looked, "took_ms", time.Since(start).Milliseconds())
	return info
}

// lookup finds a link's title and thumbnail: first what's already saved,
// then from CardyB, then from Browser Run, saving what it finds.
func (h *Hydrator) lookup(ctx context.Context, url string) page {
	key := urlKey(url)
	var p page
	var err error
	if p.title, err = h.titles.Get(ctx, url); err != nil {
		h.logger.Warn("couldn't get saved title", "url", url, "error", err)
	}
	if p.thumbnail, err = h.findThumbnail(ctx, key); err != nil {
		h.logger.Warn("couldn't check for thumbnail", "url", url, "error", err)
	}
	if p.title != "" && p.thumbnail != "" {
		return p
	}

	title, image, err := h.card(ctx, url)
	if err != nil {
		h.logger.Info("couldn't get link preview", "url", url, "error", err)
	}
	if (cmp.Or(p.title, title) == "" || cmp.Or(p.thumbnail, image) == "") && h.browser != nil && !h.browsed[url] {
		h.browsed[url] = true
		bTitle, bImage := h.browse(ctx, url)
		title, image = cmp.Or(title, bTitle), cmp.Or(image, bImage)
	}
	if p.title == "" {
		if p.title = formatTitle(title); p.title != "" {
			if err := h.titles.Put(ctx, url, p.title); err != nil {
				h.logger.Warn("couldn't save title", "url", url, "error", err)
			}
		}
	}
	if p.thumbnail == "" && image != "" {
		if p.thumbnail, err = h.saveThumbnail(ctx, key, image); err != nil {
			h.logger.Info("couldn't save thumbnail", "url", url, "image", image, "error", err)
		}
	}
	return p
}

// browse loads a page with Browser Run and returns its title and preview
// image.
func (h *Hydrator) browse(ctx context.Context, url string) (title, image string) {
	res, used, err := h.browser.Scrape(ctx, url, []string{ogTitle, ogImage, pageTitle})
	if err != nil {
		h.logger.Info("couldn't load page with browser run", "url", url, "browser_ms", used.Milliseconds(), "error", err)
		return "", ""
	}
	title = cmp.Or(res.Attr(ogTitle, "content"), res.Text(pageTitle))
	image = res.Attr(ogImage, "content")
	h.logger.Info("loaded page with browser run", "url", url, "title", title != "", "image", image != "", "browser_ms", used.Milliseconds())
	return title, image
}

// recommend picks up to three posts for each link from its most active
// posts, fetching them all from Bluesky together. A post is recommended if
// it has some text, is in English, has more than 50 likes, and its author
// doesn't already have a post recommended for the link.
func (h *Hydrator) recommend(ctx context.Context, entries []ranking.Entry) map[string][]Post {
	var uris []string
	seen := map[string]bool{}
	for _, e := range entries {
		for _, p := range e.Posts {
			if !seen[p.URI] {
				seen[p.URI] = true
				uris = append(uris, p.URI)
			}
		}
	}
	// On an error, use whatever posts did arrive.
	fetched, err := h.getPosts(ctx, uris)
	if err != nil {
		h.logger.Warn("couldn't get posts", "error", err)
	}

	out := make(map[string][]Post, len(entries))
	for _, e := range entries {
		if _, done := out[e.URL]; done {
			continue
		}
		picked := []Post{}
		authors := map[string]bool{}
		for _, candidate := range e.Posts {
			if len(picked) == maxPosts {
				break
			}
			p, ok := fetched[candidate.URI]
			if !ok {
				continue
			}
			text := formatPost(p.Record.Text)
			if text == "" || !p.english() || p.LikeCount <= minPostLikes || authors[p.Author.Handle] {
				continue
			}
			authors[p.Author.Handle] = true
			picked = append(picked, Post{AtURI: p.URI, Username: p.Author.DisplayName, Handle: p.Author.Handle, Text: text})
		}
		out[e.URL] = picked
	}
	return out
}
