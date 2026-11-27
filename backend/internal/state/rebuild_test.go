package state

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"github.com/georgemblack/blue-report/backend/internal/eventlog"
	"github.com/georgemblack/blue-report/backend/internal/ranking"
	"github.com/georgemblack/blue-report/backend/internal/store"
	"github.com/georgemblack/blue-report/backend/internal/stream"
)

// simulate produces a random but realistic stream of interactions over the
// given span: posts sharing a few dozen links, and quotes, reposts and likes
// of earlier posts (some long after the post, so they miss the cache).
// Times are random to the microsecond.
func simulate(seed uint64, n int, from time.Time, span time.Duration) []stream.Message {
	rng := rand.New(rand.NewPCG(seed, seed))
	var msgs []stream.Message
	var posts []string
	t := from
	step := span / time.Duration(n)
	for i := range n {
		t = t.Add(time.Duration(rng.Int64N(int64(2 * step))))
		did := fmt.Sprintf("did:plc:u%d", rng.IntN(300))
		rkey := fmt.Sprintf("r%d", i)
		at := t.UnixMicro()

		if len(posts) == 0 || rng.IntN(10) < 2 {
			uri := fmt.Sprintf("at://%s/app.bsky.feed.post/%s", did, rkey)
			url := fmt.Sprintf("https://site%d.com/article?utm_source=x", rng.IntN(60))
			if rng.IntN(20) == 0 {
				url = "https://media.tenor.com/x.gif" // ignored
			}
			posts = append(posts, uri)
			msgs = append(msgs, stream.Message{TimeUS: at, DID: did, RKey: rkey, Kind: stream.KindPost, SubjectURI: uri, URL: url})
			continue
		}
		// Mostly recent posts, sometimes an old one.
		idx := len(posts) - 1 - rng.IntN(min(len(posts), 40))
		if rng.IntN(10) == 0 {
			idx = rng.IntN(len(posts))
		}
		kind := []stream.Kind{stream.KindLike, stream.KindLike, stream.KindLike, stream.KindRepost, stream.KindQuote}[rng.IntN(5)]
		msgs = append(msgs, stream.Message{TimeUS: at, DID: did, RKey: rkey, Kind: kind, SubjectURI: posts[idx]})
	}
	return msgs
}

// counts strips a ranking down to what must match: links and their counts.
// (Per-post activity counts can differ slightly, since a rebuild skips
// interactions older than a week that live processing counted.)
func counts(entries []ranking.Entry) string {
	var s string
	for _, e := range entries {
		s += fmt.Sprintf("%s %v %v %v\n", e.URL, e.Hour, e.Day, e.Week)
	}
	return s
}

// Processes events live (saving files as the writer would), rebuilds a
// fresh actor from those files, and checks both have the same rankings and
// the same posts cached.
func TestRebuildMatchesLiveState(t *testing.T) {
	ctx := context.Background()
	for _, seed := range []uint64{1, 2, 3} {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			s := store.NewLocal(t.TempDir())
			live := NewProcessor()
			msgs := simulate(seed, 20_000, start0, 9*24*time.Hour) // more than a week

			// The newest saved event, which is what startup passes as "now"
			// (the resume point). Live processing's own newest time can be
			// later, since it also counts events it skipped.
			var newestSaved int64
			save := func() {
				if live.BufferLen() == 0 {
					return
				}
				data, maxT, err := eventlog.Encode(live.TakeBuffer())
				newestSaved = max(newestSaved, maxT)
				if err != nil {
					t.Fatal(err)
				}
				if err := s.Put(ctx, eventlog.Key(maxT), data, store.Meta{}); err != nil {
					t.Fatal(err)
				}
			}
			for i, m := range msgs {
				live.Handle(m)
				if i%700 == 0 {
					save()
				}
			}
			save()

			now := time.UnixMicro(newestSaved)
			rebuilt := NewActor(slowConfig(), discard)
			stats, err := rebuilt.Rebuild(ctx, s, now, 4)
			if err != nil {
				t.Fatal(err)
			}
			if stats.Files == 0 || stats.Restored == 0 {
				t.Fatalf("rebuild read nothing: %+v", stats)
			}
			if got := rebuilt.proc.Newest(); !got.Equal(now) {
				t.Fatalf("rebuilt Newest = %v, want %v", got, now)
			}

			want := live.ranking.Top(now, 1000)
			got := rebuilt.proc.ranking.Top(now, 1000)
			for _, w := range []struct {
				name      string
				want, got []ranking.Entry
			}{{"hour", want.Hour, got.Hour}, {"day", want.Day, got.Day}, {"week", want.Week, got.Week}} {
				if len(w.want) == 0 {
					t.Fatalf("%s: live ranking is empty; the test isn't exercising anything", w.name)
				}
				if counts(w.got) != counts(w.want) {
					t.Errorf("%s ranking differs:\nlive:\n%s\nrebuilt:\n%s", w.name, counts(w.want), counts(w.got))
				}
			}

			// Every post is either cached in both, with the same link, or in neither.
			cached := 0
			for _, m := range msgs {
				if m.Kind != stream.KindPost {
					continue
				}
				wantURL, wantOK := live.posts.Get(m.SubjectURI, now)
				gotURL, gotOK := rebuilt.proc.posts.Get(m.SubjectURI, now)
				if wantOK != gotOK || wantURL != gotURL {
					t.Fatalf("post %s: live (%q, %v), rebuilt (%q, %v)", m.SubjectURI, wantURL, wantOK, gotURL, gotOK)
				}
				if wantOK {
					cached++
				}
			}
			if cached == 0 {
				t.Fatal("no posts cached; the test isn't exercising the post cache")
			}
		})
	}
}

// Rebuilding twice from the same files (or from files with overlapping
// events, as after a restart) gives the same rankings.
func TestRebuildIgnoresDuplicates(t *testing.T) {
	ctx := context.Background()
	s := store.NewLocal(t.TempDir())
	live := NewProcessor()
	for _, m := range simulate(7, 2_000, start0, 2*24*time.Hour) {
		live.Handle(m)
	}
	events := live.TakeBuffer()
	// Save everything, then save the last half again in a second file.
	data, maxT, _ := eventlog.Encode(events)
	now := time.UnixMicro(maxT)
	_ = s.Put(ctx, eventlog.Key(maxT), data, store.Meta{})
	dup, _, _ := eventlog.Encode(events[len(events)/2:])
	_ = s.Put(ctx, eventlog.Key(maxT-1), dup, store.Meta{})

	rebuilt := NewActor(slowConfig(), discard)
	if _, err := rebuilt.Rebuild(ctx, s, now, 2); err != nil {
		t.Fatal(err)
	}
	want, got := live.ranking.Top(now, 1000), rebuilt.proc.ranking.Top(now, 1000)
	if counts(got.Week) != counts(want.Week) || counts(got.Hour) != counts(want.Hour) {
		t.Fatalf("rankings differ with duplicate events:\nlive:\n%s\nrebuilt:\n%s", counts(want.Week), counts(got.Week))
	}
}

// Links that were popular before a restart are sent to the resolver again
// during the rebuild, once each, even with duplicate events.
func TestRebuildSendsPopularLinks(t *testing.T) {
	ctx := context.Background()
	cfg := slowConfig()
	cfg.RedirectMinUsers = 20

	var want []string
	live := NewProcessor()
	live.popularMinUsers = cfg.RedirectMinUsers
	live.popular = func(url string) { want = append(want, url) }
	for _, m := range simulate(7, 2_000, start0, 2*24*time.Hour) {
		live.Handle(m)
	}
	if len(want) == 0 {
		t.Fatal("no popular links; the test isn't exercising anything")
	}

	events := live.TakeBuffer()
	s := store.NewLocal(t.TempDir())
	data, maxT, _ := eventlog.Encode(events)
	_ = s.Put(ctx, eventlog.Key(maxT), data, store.Meta{})
	dup, _, _ := eventlog.Encode(events[len(events)/2:])
	_ = s.Put(ctx, eventlog.Key(maxT-1), dup, store.Meta{})

	rebuilt := NewActor(cfg, discard)
	if _, err := rebuilt.Rebuild(ctx, s, time.UnixMicro(maxT), 2); err != nil {
		t.Fatal(err)
	}
	var got []string
	for len(rebuilt.Popular()) > 0 {
		got = append(got, <-rebuilt.Popular())
	}
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("sent %v, want %v", got, want)
	}
}

func TestRebuildEmptyStore(t *testing.T) {
	a := NewActor(slowConfig(), discard)
	stats, err := a.Rebuild(context.Background(), store.NewLocal(t.TempDir()), start0, 2)
	if err != nil || stats.Files != 0 {
		t.Fatalf("stats = %+v, err = %v", stats, err)
	}
}

// BenchmarkRebuildFile measures decoding one saved file and restoring its
// events, the work a startup rebuild repeats for every file in the last
// week. Reports the cost per event.
func BenchmarkRebuildFile(b *testing.B) {
	live := NewProcessor()
	for _, m := range simulate(11, 200_000, start0, 6*time.Hour) {
		live.Handle(m)
	}
	events := live.TakeBuffer()
	data, maxT, err := eventlog.Encode(events)
	if err != nil {
		b.Fatal(err)
	}
	now := time.UnixMicro(maxT)
	b.ResetTimer()
	for b.Loop() {
		decoded, err := eventlog.Decode(data)
		if err != nil {
			b.Fatal(err)
		}
		NewProcessor().Restore(decoded, now)
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*len(events)), "ns/event")
}
