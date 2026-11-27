package resolve

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"testing"
	"time"
)

var ctx = context.Background()

type merge struct{ source, destination string }

type fakeMerger struct{ merges []merge }

func (f *fakeMerger) Merge(_ context.Context, source, destination string) error {
	f.merges = append(f.merges, merge{source, destination})
	return nil
}

const (
	short   = "https://nyti.ms/abc"
	article = "https://www.nytimes.com/article.html"
)

var now = time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)

// newTest returns a resolver whose redirects come from the given map, and
// counts how many links it follows.
func newTest(store Store, redirects map[string]string) (*Resolver, *fakeMerger, *int) {
	merger := &fakeMerger{}
	r := New(store, merger, slog.New(slog.NewTextHandler(io.Discard, nil)))
	followed := 0
	r.follow = func(url string) string { followed++; return redirects[url] }
	r.now = func() time.Time { return now }
	return r, merger, &followed
}

// A new redirect is followed, merged and saved; the next time (such as after
// a restart) the saved answer is used.
func TestRedirect(t *testing.T) {
	store := NewMemoryStore()
	r, merger, followed := newTest(store, map[string]string{short: article + "?smid=share"})
	r.resolve(ctx, short)
	r.resolve(ctx, short)

	want := []merge{{short, article}, {short, article}}
	if !slices.Equal(merger.merges, want) {
		t.Fatalf("merges = %v, want %v", merger.merges, want)
	}
	if *followed != 1 {
		t.Fatalf("followed %d times, want 1", *followed)
	}
	if d, found, _ := store.Get(ctx, short, now.Add(keepFor-time.Second)); !found || d != article {
		t.Fatalf("saved = %q, %v", d, found)
	}
}

// A link that doesn't redirect isn't merged, and that's saved too.
func TestNoRedirect(t *testing.T) {
	store := NewMemoryStore()
	r, merger, followed := newTest(store, map[string]string{
		article: article + "?utm_source=x", // cleans back to itself
	})
	r.resolve(ctx, article)
	r.resolve(ctx, article)
	if len(merger.merges) != 0 || *followed != 1 {
		t.Fatalf("merges = %v, followed %d times", merger.merges, *followed)
	}
}

// After 10 days a saved answer is no longer used.
func TestSavedAnswerExpires(t *testing.T) {
	store := NewMemoryStore()
	store.Put(ctx, short, "https://old.example.com", now)
	r, merger, _ := newTest(store, map[string]string{short: article})
	r.resolve(ctx, short)
	if !slices.Equal(merger.merges, []merge{{short, article}}) {
		t.Fatalf("merges = %v", merger.merges)
	}
}

func TestAppleNewsSkipped(t *testing.T) {
	r, merger, followed := newTest(NewMemoryStore(), nil)
	r.resolve(ctx, "https://apple.news/abc")
	if len(merger.merges) != 0 || *followed != 0 {
		t.Fatalf("merges = %v, followed %d times", merger.merges, *followed)
	}
}

func TestRun(t *testing.T) {
	r, merger, _ := newTest(NewMemoryStore(), map[string]string{short: article})
	links := make(chan string, 1)
	links <- short
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { r.Run(runCtx, links); close(done) }()
	// Run handles one link at a time, so once a second send is taken the
	// first has been handled.
	links <- article
	links <- article
	cancel()
	<-done
	if len(merger.merges) == 0 || merger.merges[0] != (merge{short, article}) {
		t.Fatalf("merges = %v", merger.merges)
	}
}
