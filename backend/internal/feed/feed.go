// Package feed keeps a feed of the day's top links, published as Atom and
// JSON Feed files. It follows v1's rules: the day's top link is added unless
// it's already in the feed or another link was added in the last 12 hours,
// and entries are kept for 90 days.
package feed

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/gorilla/feeds"

	"github.com/georgemblack/blue-report/backend/internal/store"
	"github.com/georgemblack/blue-report/backend/internal/urltools"
)

const (
	gap     = 12 * time.Hour      // at least this long between entries
	keepFor = 90 * 24 * time.Hour // how long entries stay in the feed

	AtomKey = "feeds/top-day.xml"
	JSONKey = "feeds/top-day.json"

	icon = "https://theblue.report/icons/web-app-manifest-512x512.png"
)

// Feed adds entries and publishes the feed files. It's used by the
// publisher only, one call at a time.
type Feed struct {
	entries Store
	files   store.Store
	baseURL string // public address of files, e.g. "https://data.theblue.report/engine/"
	logger  *slog.Logger

	published bool // whether the files have been published since startup
}

func New(entries Store, files store.Store, baseURL string, logger *slog.Logger) *Feed {
	return &Feed{entries: entries, files: files, baseURL: baseURL, logger: logger}
}

// Update adds top (the day's top link) to the feed if the rules allow, then
// publishes the feed files if anything changed. The files are also
// published the first time it's called, so they're always up to date after
// a deploy.
func (f *Feed) Update(ctx context.Context, top Entry, now time.Time) error {
	entries, err := f.entries.List(ctx)
	if err != nil {
		return err
	}
	if f.shouldAdd(entries, top, now) {
		top.AddedAt = now
		if err := f.entries.Add(ctx, top, now.Add(keepFor)); err != nil {
			return err
		}
		entries = append(entries, top)
		f.logger.Info("added feed entry", "url", top.URL, "title", top.Title)
	} else if f.published {
		return nil
	}

	// Newest first. Entries past 90 days may not be deleted yet.
	entries = slices.DeleteFunc(entries, func(e Entry) bool { return now.Sub(e.AddedAt) > keepFor })
	slices.SortFunc(entries, func(a, b Entry) int { return b.AddedAt.Compare(a.AddedAt) })
	if err := f.publish(ctx, entries, now); err != nil {
		return err
	}
	f.published = true
	return nil
}

func (f *Feed) shouldAdd(entries []Entry, top Entry, now time.Time) bool {
	// Links without a title yet are left for a later publish.
	if top.URL == "" || top.Title == "" {
		return false
	}
	for _, e := range entries {
		if e.URL == top.URL || now.Sub(e.AddedAt) < gap {
			return false
		}
	}
	return true
}

func (f *Feed) publish(ctx context.Context, entries []Entry, now time.Time) error {
	atom, err := f.atom(entries, now)
	if err != nil {
		return err
	}
	jsonFeed, err := f.json(entries)
	if err != nil {
		return err
	}
	cache := "public, max-age=600" // 10 minutes, as in v1
	if err := f.files.Put(ctx, AtomKey, []byte(atom), store.Meta{ContentType: "application/atom+xml", CacheControl: cache}); err != nil {
		return fmt.Errorf("publishing atom feed: %w", err)
	}
	if err := f.files.Put(ctx, JSONKey, jsonFeed, store.Meta{ContentType: "application/feed+json", CacheControl: cache}); err != nil {
		return fmt.Errorf("publishing json feed: %w", err)
	}
	return nil
}

// atom builds the Atom feed, the same way v1 does.
func (f *Feed) atom(entries []Entry, now time.Time) (string, error) {
	feed := feeds.AtomFeed{
		Xmlns:   "http://www.w3.org/2005/Atom",
		Title:   "The Blue Report",
		Link:    &feeds.AtomLink{Href: f.baseURL + AtomKey, Rel: "self"},
		Id:      f.baseURL + AtomKey,
		Icon:    icon,
		Updated: now.UTC().Format(time.RFC3339),
	}
	for _, e := range entries {
		feed.Entries = append(feed.Entries, &feeds.AtomEntry{
			Id:      e.URL,
			Title:   e.Title,
			Links:   []feeds.AtomLink{{Href: e.URL, Rel: "alternate"}},
			Content: &feeds.AtomContent{Content: content(e), Type: "html"},
			Author:  &feeds.AtomAuthor{AtomPerson: feeds.AtomPerson{Name: urltools.Hostname(e.URL)}},
			Updated: e.AddedAt.UTC().Format(time.RFC3339),
		})
	}
	atom, err := feeds.ToXML(&feed)
	if err != nil {
		return "", fmt.Errorf("building atom feed: %w", err)
	}
	return atom, nil
}

// json builds the JSON Feed (jsonfeed.org), the same way v1 does.
func (f *Feed) json(entries []Entry) ([]byte, error) {
	type item struct {
		ID            string `json:"id"`
		URL           string `json:"url"`
		Title         string `json:"title"`
		ContentHTML   string `json:"content_html"`
		DatePublished string `json:"date_published"`
	}
	feed := struct {
		Version     string `json:"version"`
		Title       string `json:"title"`
		HomePageURL string `json:"home_page_url"`
		FeedURL     string `json:"feed_url"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
		Language    string `json:"language"`
		Items       []item `json:"items"`
	}{
		Version:     "https://jsonfeed.org/version/1.1",
		Title:       "The Blue Report",
		HomePageURL: "https://theblue.report",
		FeedURL:     f.baseURL + JSONKey,
		Description: "The top links on Bluesky over the past day",
		Icon:        icon,
		Language:    "en",
		Items:       []item{},
	}
	for _, e := range entries {
		feed.Items = append(feed.Items, item{
			ID:            e.URL,
			URL:           e.URL,
			Title:         e.Title,
			ContentHTML:   content(e),
			DatePublished: e.AddedAt.UTC().Format(time.RFC3339),
		})
	}
	data, err := json.Marshal(feed)
	if err != nil {
		return nil, fmt.Errorf("building json feed: %w", err)
	}
	return data, nil
}

// content is an entry's body: the link, then each recommended post.
func content(e Entry) string {
	esc := html.EscapeString
	var b strings.Builder
	fmt.Fprintf(&b, `<p>Trending on Bluesky: <a href="%s">%s</a></p>`, esc(e.URL), esc(e.Title))
	for _, p := range e.Posts {
		fmt.Fprintf(&b, `<p>Post by <a href="https://bsky.app/profile/%s">@%s</a></p>`, esc(p.Handle), esc(p.Handle))
		fmt.Fprintf(&b, `<blockquote>%s</blockquote>`, esc(p.Text))
		fmt.Fprintf(&b, `<p><a href="%s">View Post</a></p>`, esc(postURL(p)))
	}
	return b.String()
}

// postURL turns a post's at:// address into its bsky.app address.
func postURL(p Post) string {
	rkey := p.AtURI[strings.LastIndex(p.AtURI, "/")+1:]
	return fmt.Sprintf("https://bsky.app/profile/%s/post/%s", p.Handle, rkey)
}
