package hydrate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"github.com/georgemblack/blue-report/backend/internal/ranking"
	"github.com/georgemblack/blue-report/backend/internal/store"
)

var (
	ctx     = context.Background()
	discard = slog.New(slog.NewTextHandler(io.Discard, nil))
)

// A PNG's first bytes, enough for http.DetectContentType.
var pngBytes = []byte("\x89PNG\r\n\x1a\n0000")

// fakeWeb stands in for CardyB, a news site's preview image and Bluesky's
// API, and counts the requests it gets.
type fakeWeb struct {
	mu      sync.Mutex
	cards   map[string][2]string // page URL -> title, image path
	posts   map[string]blueskyPost
	cardyB  int
	batches [][]string
	server  *httptest.Server
}

func newFakeWeb(t *testing.T) *fakeWeb {
	f := &fakeWeb{cards: map[string][2]string{}, posts: map[string]blueskyPost{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/extract", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.cardyB++
		card, ok := f.cards[r.URL.Query().Get("url")]
		if !ok {
			http.Error(w, "no card", http.StatusBadRequest)
			return
		}
		image := ""
		if card[1] != "" {
			image = f.server.URL + card[1]
		}
		json.NewEncoder(w).Encode(map[string]string{"title": card[0], "image": image})
	})
	mux.HandleFunc("/image.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream") // so the bytes decide
		w.Write(pngBytes)
	})
	mux.HandleFunc("/xrpc/app.bsky.feed.getPosts", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		uris := r.URL.Query()["uris"]
		f.batches = append(f.batches, uris)
		var out struct {
			Posts []blueskyPost `json:"posts"`
		}
		for _, u := range uris {
			if p, ok := f.posts[u]; ok {
				out.Posts = append(out.Posts, p)
			}
		}
		json.NewEncoder(w).Encode(out)
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeWeb) cardyBCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cardyB
}

func (f *fakeWeb) addPost(uri, handle, text string, likes int, langs ...string) {
	p := blueskyPost{URI: uri, LikeCount: likes}
	p.Author.Handle = handle
	p.Author.DisplayName = "Name of " + handle
	p.Record.Text = text
	p.Record.Langs = langs
	f.posts[uri] = p
}

func newTestHydrator(f *fakeWeb, titles Titles, thumbs store.Store) *Hydrator {
	h := New(titles, thumbs, discard)
	h.cardyBURL = f.server.URL
	h.cardyBDelay = 0
	h.blueskyURL = f.server.URL
	return h
}

const nyt = "https://www.nytimes.com/a.html"

// The key must match v1's util.Hash, or v1's titles and thumbnails won't be
// found.
func TestURLKeyMatchesV1(t *testing.T) {
	if got := urlKey(nyt); got != "56a928a774a434bc" {
		t.Fatalf("urlKey = %s", got)
	}
}

func TestFormat(t *testing.T) {
	if got := formatTitle("🚨 BREAKING: Something happened "); got != "BREAKING: Something happened" {
		// As in v1, the prefix is only removed when it comes first.
		t.Errorf("formatTitle = %q", got)
	}
	if got := formatTitle("BREAKING: Something happened"); got != "Something happened" {
		t.Errorf("formatTitle = %q", got)
	}
	if got := formatPost("NEW: Read this\n\n  now apnews.com/article/trum..."); got != "Read this now" {
		t.Errorf("formatPost = %q", got)
	}
	if got := formatPost("www.example.com/some-arti..."); got != "" {
		t.Errorf("formatPost = %q", got)
	}
}

// A new link gets its title and image from CardyB; both are saved where v1
// keeps them, and the next publish doesn't look it up again.
func TestNewLink(t *testing.T) {
	f := newFakeWeb(t)
	f.cards[nyt] = [2]string{"BREAKING: A story", "/image.png"}
	titles := NewMemoryTitles()
	thumbs := store.NewLocal(t.TempDir())
	h := newTestHydrator(f, titles, thumbs)

	info := h.Hydrate(ctx, []ranking.Entry{{URL: nyt}})
	want := Info{Title: "A story", ThumbnailURL: "https://data.theblue.report/thumbnails/56a928a774a434bc.png", Posts: []Post{}}
	if got := info[nyt]; got.Title != want.Title || got.ThumbnailURL != want.ThumbnailURL || got.Posts == nil {
		t.Fatalf("info = %+v, want %+v", got, want)
	}
	if saved, _ := titles.Get(ctx, nyt); saved != "A story" {
		t.Errorf("saved title = %q", saved)
	}
	if body, err := thumbs.Get(ctx, "thumbnails/56a928a774a434bc.png"); err != nil || string(body) != string(pngBytes) {
		t.Errorf("saved thumbnail = %q, %v", body, err)
	}

	h.Hydrate(ctx, []ranking.Entry{{URL: nyt}})
	if n := f.cardyBCalls(); n != 1 {
		t.Errorf("CardyB called %d times, want 1", n)
	}
}

// A link v1 already has a title and thumbnail for needs no CardyB call,
// including after a restart (a new Hydrator).
func TestSavedLink(t *testing.T) {
	f := newFakeWeb(t)
	titles := NewMemoryTitles()
	titles.Put(ctx, nyt, "Saved title")
	thumbs := store.NewLocal(t.TempDir())
	thumbs.Put(ctx, "thumbnails/56a928a774a434bc.jpg", []byte("jpg"), store.Meta{})

	info := newTestHydrator(f, titles, thumbs).Hydrate(ctx, []ranking.Entry{{URL: nyt}})
	if got := info[nyt]; got.Title != "Saved title" || got.ThumbnailURL != "https://data.theblue.report/thumbnails/56a928a774a434bc.jpg" {
		t.Fatalf("info = %+v", got)
	}
	if n := f.cardyBCalls(); n != 0 {
		t.Errorf("CardyB called %d times", n)
	}
}

// When CardyB has nothing, the link is left empty and tried again on the
// next publish.
func TestNoTitleRetried(t *testing.T) {
	f := newFakeWeb(t)
	h := newTestHydrator(f, NewMemoryTitles(), store.NewLocal(t.TempDir()))
	for range 2 {
		if info := h.Hydrate(ctx, []ranking.Entry{{URL: nyt}}); info[nyt].Title != "" || info[nyt].ThumbnailURL != "" {
			t.Fatalf("info = %+v", info[nyt])
		}
	}
	if n := f.cardyBCalls(); n != 2 {
		t.Errorf("CardyB called %d times, want 2", n)
	}
}

func TestRecommendedPosts(t *testing.T) {
	f := newFakeWeb(t)
	f.addPost("at://a/1", "alice", "Too few likes", 50, "en")
	f.addPost("at://b/1", "bob", "Worth reading", 51, "en")
	f.addPost("at://b/2", "bob", "Bob again", 500, "en")
	f.addPost("at://c/1", "carol", "Pas en anglais", 500, "fr")
	f.addPost("at://d/1", "dave", "nytimes.com/a.ht...", 500, "en") // only a link
	f.addPost("at://e/1", "erin", "Second pick", 100, "en")
	f.addPost("at://g/1", "gina", "Third pick", 100, "en")
	f.addPost("at://h/1", "hank", "One too many", 100, "en")
	// at://x/1 was deleted, so Bluesky doesn't return it.

	var candidates []ranking.Post
	for _, uri := range []string{"at://x/1", "at://a/1", "at://b/1", "at://b/2", "at://c/1", "at://d/1", "at://e/1", "at://g/1", "at://h/1"} {
		candidates = append(candidates, ranking.Post{URI: uri})
	}
	f.cards[nyt] = [2]string{"A story", ""}
	h := newTestHydrator(f, NewMemoryTitles(), store.NewLocal(t.TempDir()))
	// The same link in two lists: its posts are only fetched once.
	entry := ranking.Entry{URL: nyt, Posts: candidates}
	info := h.Hydrate(ctx, []ranking.Entry{entry, entry})

	var got []string
	for _, p := range info[nyt].Posts {
		got = append(got, p.Handle+": "+p.Text)
	}
	want := []string{"bob: Worth reading", "erin: Second pick", "gina: Third pick"}
	if !slices.Equal(got, want) {
		t.Fatalf("posts = %q, want %q", got, want)
	}
	if p := info[nyt].Posts[0]; p.AtURI != "at://b/1" || p.Username != "Name of bob" {
		t.Errorf("first post = %+v", p)
	}
	if len(f.batches) != 1 || len(f.batches[0]) != len(candidates) {
		t.Errorf("batches = %v", f.batches)
	}
}

// Bluesky allows 25 posts per request.
func TestGetPostsBatches(t *testing.T) {
	f := newFakeWeb(t)
	var uris []string
	for i := range 60 {
		uri := fmt.Sprintf("at://did/%d", i)
		uris = append(uris, uri)
		f.addPost(uri, "h", "t", 0, "en")
	}
	posts, err := newTestHydrator(f, NewMemoryTitles(), store.NewLocal(t.TempDir())).getPosts(ctx, uris)
	if err != nil || len(posts) != 60 {
		t.Fatalf("got %d posts, %v", len(posts), err)
	}
	var sizes []int
	for _, b := range f.batches {
		sizes = append(sizes, len(b))
	}
	if !slices.Equal(sizes, []int{25, 25, 10}) {
		t.Errorf("batch sizes = %v", sizes)
	}
}
