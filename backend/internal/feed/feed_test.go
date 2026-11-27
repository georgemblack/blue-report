package feed

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/georgemblack/blue-report/backend/internal/store"
)

var (
	ctx = context.Background()
	t0  = time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
)

func newTest(t *testing.T) (*Feed, *MemoryStore, store.Store) {
	entries := NewMemoryStore()
	files := store.NewLocal(t.TempDir())
	return New(entries, files, "https://data.theblue.report/engine/", slog.New(slog.NewTextHandler(io.Discard, nil))), entries, files
}

func link(n string) Entry {
	return Entry{URL: "https://example.com/" + n, Title: "Story " + n}
}

func urls(t *testing.T, s *MemoryStore) []string {
	t.Helper()
	entries, _ := s.List(ctx)
	var out []string
	for _, e := range entries {
		out = append(out, e.URL)
	}
	return out
}

// jsonItems reads back the published JSON feed's item URLs.
func jsonItems(t *testing.T, files store.Store) []string {
	t.Helper()
	data, err := files.Get(ctx, JSONKey)
	if err != nil {
		t.Fatal(err)
	}
	var feed struct {
		Items []struct{ URL string } `json:"items"`
	}
	if err := json.Unmarshal(data, &feed); err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, i := range feed.Items {
		out = append(out, i.URL)
	}
	return out
}

func TestEntriesAtLeast12HoursApart(t *testing.T) {
	f, entries, files := newTest(t)
	f.Update(ctx, link("a"), t0)
	f.Update(ctx, link("b"), t0.Add(11*time.Hour)) // too soon
	f.Update(ctx, link("a"), t0.Add(13*time.Hour)) // already in the feed
	f.Update(ctx, link("c"), t0.Add(14*time.Hour))

	if got := strings.Join(urls(t, entries), " "); got != "https://example.com/a https://example.com/c" {
		t.Fatalf("entries = %s", got)
	}
	// Newest first.
	if got := strings.Join(jsonItems(t, files), " "); got != "https://example.com/c https://example.com/a" {
		t.Fatalf("published items = %s", got)
	}
}

func TestLinkWithoutTitleWaits(t *testing.T) {
	f, entries, _ := newTest(t)
	f.Update(ctx, Entry{URL: "https://example.com/a"}, t0)
	if len(urls(t, entries)) != 0 {
		t.Fatal("added a link without a title")
	}
}

// The files are published on the first update after startup even when
// nothing's added, and after that only when an entry is added.
func TestPublishesOnStartupAndChanges(t *testing.T) {
	f, entries, files := newTest(t)
	entries.Add(ctx, Entry{URL: "https://example.com/old", Title: "Old", AddedAt: t0}, t0)

	f.Update(ctx, link("a"), t0.Add(time.Hour)) // too soon to add
	if got := jsonItems(t, files); len(got) != 1 {
		t.Fatalf("published items = %v", got)
	}
	files.Put(ctx, JSONKey, []byte("{}"), store.Meta{}) // so a republish would show
	f.Update(ctx, link("a"), t0.Add(2*time.Hour))
	if got := jsonItems(t, files); len(got) != 0 {
		t.Fatalf("republished without a change: %v", got)
	}
	f.Update(ctx, link("a"), t0.Add(13*time.Hour))
	if got := jsonItems(t, files); len(got) != 2 {
		t.Fatalf("published items = %v", got)
	}
}

func TestOldEntriesLeftOut(t *testing.T) {
	f, entries, files := newTest(t)
	entries.Add(ctx, Entry{URL: "https://example.com/old", Title: "Old", AddedAt: t0.Add(-91 * 24 * time.Hour)}, t0)
	f.Update(ctx, link("a"), t0)
	if got := jsonItems(t, files); len(got) != 1 || got[0] != "https://example.com/a" {
		t.Fatalf("published items = %v", got)
	}
}

func TestContent(t *testing.T) {
	e := Entry{
		URL:   "https://example.com/a",
		Title: "Cats & dogs",
		Posts: []Post{{AtURI: "at://did:plc:x/app.bsky.feed.post/3abc", Handle: "alice.bsky.social", Text: "<b>wow</b>"}},
	}
	want := `<p>Trending on Bluesky: <a href="https://example.com/a">Cats &amp; dogs</a></p>` +
		`<p>Post by <a href="https://bsky.app/profile/alice.bsky.social">@alice.bsky.social</a></p>` +
		`<blockquote>&lt;b&gt;wow&lt;/b&gt;</blockquote>` +
		`<p><a href="https://bsky.app/profile/alice.bsky.social/post/3abc">View Post</a></p>`
	if got := content(e); got != want {
		t.Fatalf("content =\n%s\nwant\n%s", got, want)
	}
}

func TestAtom(t *testing.T) {
	f, _, files := newTest(t)
	f.Update(ctx, link("a"), t0)
	data, err := files.Get(ctx, AtomKey)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<link href="https://data.theblue.report/engine/feeds/top-day.xml" rel="self">`,
		`<id>https://example.com/a</id>`,
		`<name>example.com</name>`,
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("atom feed is missing %s:\n%s", want, data)
		}
	}
}
