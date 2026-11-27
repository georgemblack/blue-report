package publish

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/georgemblack/blue-report/backend/internal/feed"
	"github.com/georgemblack/blue-report/backend/internal/hydrate"
	"github.com/georgemblack/blue-report/backend/internal/ranking"
	"github.com/georgemblack/blue-report/backend/internal/state"
	"github.com/georgemblack/blue-report/backend/internal/store"
)

var (
	ctx     = context.Background()
	discard = slog.New(slog.NewTextHandler(io.Discard, nil))
	when    = time.Date(2026, 11, 26, 15, 4, 5, 0, time.UTC)
)

func sampleResult() state.TopResult {
	nyt := ranking.Entry{
		URL:  "https://www.nytimes.com/a.html",
		Hour: ranking.Counts{Posts: 1, Likes: 4},
		Day:  ranking.Counts{Posts: 2, Reposts: 1, Likes: 9},
		Week: ranking.Counts{Posts: 3, Reposts: 2, Likes: 20},
	}
	bbc := ranking.Entry{URL: "https://www.bbc.co.uk/b", Week: ranking.Counts{Likes: 5}}
	return state.TopResult{
		Now:     when,
		Ranking: ranking.Result{Hour: []ranking.Entry{nyt}, Day: []ranking.Entry{nyt}, Week: []ranking.Entry{nyt, bbc}},
	}
}

// The JSON matches v1's top-links.json exactly, field for field.
func TestSnapshotJSON(t *testing.T) {
	data, err := json.Marshal(BuildSnapshot(sampleResult(), nil, when))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"generated_at":"2026-11-26T15:04:05Z",` +
		`"top_hour":[{"rank":1,"url":"https://www.nytimes.com/a.html","title":"(No Title)","thumbnail_url":"","post_count":3,"repost_count":2,"like_count":20,"recommended_posts":[]}],` +
		`"top_day":[{"rank":1,"url":"https://www.nytimes.com/a.html","title":"(No Title)","thumbnail_url":"","post_count":3,"repost_count":2,"like_count":20,"recommended_posts":[]}],` +
		`"top_week":[{"rank":1,"url":"https://www.nytimes.com/a.html","title":"(No Title)","thumbnail_url":"","post_count":3,"repost_count":2,"like_count":20,"recommended_posts":[]},` +
		`{"rank":2,"url":"https://www.bbc.co.uk/b","title":"(No Title)","thumbnail_url":"","post_count":0,"repost_count":0,"like_count":5,"recommended_posts":[]}]}`
	if string(data) != want {
		t.Fatalf("got\n%s\nwant\n%s", data, want)
	}
}

// Empty lists are [] rather than null, so the website can loop over them.
func TestEmptySnapshot(t *testing.T) {
	data, _ := json.Marshal(BuildSnapshot(state.TopResult{}, nil, when))
	want := `{"generated_at":"2026-11-26T15:04:05Z","top_hour":[],"top_day":[],"top_week":[]}`
	if string(data) != want {
		t.Fatalf("got %s", data)
	}
}

type fakeRanker struct {
	mu    sync.Mutex
	calls int
	err   error
}

func (f *fakeRanker) Top(context.Context) (state.TopResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return state.TopResult{}, f.err
	}
	return sampleResult(), nil
}

func (f *fakeRanker) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func TestPublish(t *testing.T) {
	s := store.NewLocal(t.TempDir())
	p := NewPublisher(&fakeRanker{}, nil, nil, s, 5*time.Minute, discard)
	p.now = func() time.Time { return when }
	if err := p.Publish(ctx); err != nil {
		t.Fatal(err)
	}
	data, err := s.Get(ctx, TopLinksKey)
	if err != nil {
		t.Fatal(err)
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		t.Fatal(err)
	}
	if snap.GeneratedAt != "2026-11-26T15:04:05Z" || len(snap.TopWeek) != 2 {
		t.Fatalf("published %+v", snap)
	}
}

// recordingStore remembers the metadata of the last Put.
type recordingStore struct {
	store.Store
	meta store.Meta
}

func (r *recordingStore) Put(ctx context.Context, key string, body []byte, meta store.Meta) error {
	r.meta = meta
	return r.Store.Put(ctx, key, body, meta)
}

func TestPublishMeta(t *testing.T) {
	s := &recordingStore{Store: store.NewLocal(t.TempDir())}
	if err := NewPublisher(&fakeRanker{}, nil, nil, s, 5*time.Minute, discard).Publish(ctx); err != nil {
		t.Fatal(err)
	}
	if want := (store.Meta{ContentType: "application/json", CacheControl: "public, max-age=300"}); s.meta != want {
		t.Fatalf("meta = %+v, want %+v", s.meta, want)
	}
}

func TestPublishError(t *testing.T) {
	p := NewPublisher(&fakeRanker{err: state.ErrStopped}, nil, nil, store.NewLocal(t.TempDir()), time.Minute, discard)
	if err := p.Publish(ctx); !errors.Is(err, state.ErrStopped) {
		t.Fatalf("err = %v", err)
	}
}

// Run publishes straight away, then on each tick, and keeps going after
// failures.
func TestRun(t *testing.T) {
	r := &fakeRanker{err: errors.New("not ready")}
	p := NewPublisher(r, nil, nil, store.NewLocal(t.TempDir()), 10*time.Millisecond, discard)
	c, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { p.Run(c); close(done) }()

	deadline := time.Now().Add(5 * time.Second)
	for r.count() < 3 {
		if time.Now().After(deadline) {
			t.Fatalf("only %d publishes", r.count())
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run didn't stop")
	}
}

// Titles, thumbnails and recommended posts from the hydrator end up in every
// list the link is in, with posts ranked from 1.
func TestSnapshotWithInfo(t *testing.T) {
	info := map[string]hydrate.Info{
		"https://www.nytimes.com/a.html": {
			Title:        "A story",
			ThumbnailURL: "https://data.theblue.report/thumbnails/x.png",
			Posts: []hydrate.Post{
				{AtURI: "at://a/1", Username: "Alice", Handle: "alice.bsky.social", Text: "Read this"},
				{AtURI: "at://b/1", Username: "Bob", Handle: "bob.bsky.social", Text: "And this"},
			},
		},
	}
	snap := BuildSnapshot(sampleResult(), info, when)
	for _, list := range [][]Link{snap.TopHour, snap.TopDay, snap.TopWeek} {
		got := list[0]
		if got.Title != "A story" || got.ThumbnailURL != "https://data.theblue.report/thumbnails/x.png" ||
			len(got.RecommendedPosts) != 2 || got.RecommendedPosts[1] != (Post{Rank: 2, AtURI: "at://b/1", Username: "Bob", Handle: "bob.bsky.social", Text: "And this"}) {
			t.Fatalf("link = %+v", got)
		}
	}
	// The BBC link has no info: fallback title, no posts.
	if bbc := snap.TopWeek[1]; bbc.Title != FallbackTitle || bbc.ThumbnailURL != "" || bbc.RecommendedPosts == nil {
		t.Fatalf("bbc = %+v", bbc)
	}
}

type fakeHydrator struct{ got []ranking.Entry }

func (f *fakeHydrator) Hydrate(_ context.Context, entries []ranking.Entry) map[string]hydrate.Info {
	f.got = entries
	return map[string]hydrate.Info{"https://www.bbc.co.uk/b": {Title: "BBC story"}}
}

// The publisher hydrates every link it publishes.
func TestPublishHydrates(t *testing.T) {
	s := store.NewLocal(t.TempDir())
	h := &fakeHydrator{}
	if err := NewPublisher(&fakeRanker{}, h, nil, s, time.Minute, discard).Publish(ctx); err != nil {
		t.Fatal(err)
	}
	if len(h.got) != 4 { // nyt in all three lists, plus bbc
		t.Fatalf("hydrated %d entries", len(h.got))
	}
	data, _ := s.Get(ctx, TopLinksKey)
	var snap Snapshot
	json.Unmarshal(data, &snap)
	if snap.TopWeek[1].Title != "BBC story" {
		t.Fatalf("week = %+v", snap.TopWeek)
	}
}

type fakeFeed struct{ got []feed.Entry }

func (f *fakeFeed) Update(_ context.Context, top feed.Entry, _ time.Time) error {
	f.got = append(f.got, top)
	return nil
}

// The day's top link is offered to the feed. Without a real title, its
// title is left empty so the feed waits for one.
func TestPublishOffersTopDayLink(t *testing.T) {
	f := &fakeFeed{}
	if err := NewPublisher(&fakeRanker{}, &fakeHydrator{}, f, store.NewLocal(t.TempDir()), time.Minute, discard).Publish(ctx); err != nil {
		t.Fatal(err)
	}
	if len(f.got) != 1 || f.got[0].URL != sampleResult().Ranking.Day[0].URL || f.got[0].Title != "" {
		t.Fatalf("feed got %+v", f.got)
	}
}
