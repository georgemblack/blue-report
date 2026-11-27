package state

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/georgemblack/blue-report/backend/internal/event"
	"github.com/georgemblack/blue-report/backend/internal/stream"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

// slowConfig has timers that won't fire during a test unless it waits.
func slowConfig() Config {
	c := DefaultConfig()
	c.WriteInterval = time.Hour
	c.SweepInterval = time.Hour
	c.StatsInterval = time.Hour
	return c
}

type harness struct {
	actor   *Actor
	events  chan stream.Message
	batches chan []event.Event
	done    chan struct{}
}

func start(t *testing.T, cfg Config) *harness {
	t.Helper()
	h := &harness{
		actor:   NewActor(cfg, discard),
		events:  make(chan stream.Message, 100),
		batches: make(chan []event.Event, 100),
		done:    make(chan struct{}),
	}
	go func() {
		h.actor.Run(h.events, h.batches)
		close(h.done)
	}()
	t.Cleanup(h.stop)
	return h
}

// stop closes the events channel (as the reader does at shutdown) and waits.
func (h *harness) stop() {
	select {
	case <-h.done:
		return
	default:
	}
	close(h.events)
	<-h.done
}

func (h *harness) top(t *testing.T) TopResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := h.actor.Top(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// waitTop polls Top until cond is true. The actor's select picks randomly
// among ready channels, so a Top request can be answered before events sent
// earlier have been handled.
func (h *harness) waitTop(t *testing.T, cond func(TopResult) bool) TopResult {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		res := h.top(t)
		if cond(res) {
			return res
		}
		if time.Now().After(deadline) {
			t.Fatalf("condition never met; last result: %+v", res)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func weekLikes(res TopResult, url string) int {
	for _, e := range res.Ranking.Week {
		if e.URL == url {
			return e.Week.Likes
		}
	}
	return -1
}

// drain returns all batches written so far, after the actor has stopped.
func (h *harness) drain() [][]event.Event {
	var out [][]event.Event
	for b := range h.batches {
		out = append(out, b)
	}
	return out
}

func likes(n int, subject string, at time.Time) []stream.Message {
	var msgs []stream.Message
	for i := range n {
		msgs = append(msgs, interaction(stream.KindLike, fmt.Sprintf("did:plc:u%d", i), fmt.Sprintf("l%d", i), subject, at))
	}
	return msgs
}

func TestActorRanksEvents(t *testing.T) {
	h := start(t, slowConfig())
	h.events <- alicePosts(start0)
	for _, m := range likes(3, alicePost, start0.Add(time.Minute)) {
		h.events <- m
	}

	res := h.waitTop(t, func(r TopResult) bool { return weekLikes(r, articleURL) == 3 })
	if !res.Now.Equal(start0.Add(time.Minute)) {
		t.Fatalf("Now = %v", res.Now)
	}
	if len(res.Ranking.Hour) != 1 || res.Ranking.Hour[0].URL != articleURL || res.Ranking.Hour[0].Hour.Likes != 3 {
		t.Fatalf("hour ranking = %+v", res.Ranking.Hour)
	}
}

// A link is sent to the resolver once, at the moment it reaches the
// threshold, and not again while it stays above it.
func TestActorSendsPopularLinks(t *testing.T) {
	cfg := slowConfig()
	cfg.RedirectMinUsers = 3
	h := start(t, cfg)
	h.events <- alicePosts(start0)
	h.events <- likes(1, alicePost, start0)[0]
	h.waitTop(t, func(r TopResult) bool { return weekLikes(r, articleURL) == 1 })
	select {
	case url := <-h.actor.Popular():
		t.Fatalf("sent %q below the threshold", url)
	default:
	}

	for _, m := range likes(3, alicePost, start0) {
		h.events <- m // the first like is a repeat; the others add users
	}
	h.waitTop(t, func(r TopResult) bool { return weekLikes(r, articleURL) == 3 })
	if url := <-h.actor.Popular(); url != articleURL {
		t.Fatalf("sent %q", url)
	}
	select {
	case url := <-h.actor.Popular():
		t.Fatalf("sent %q twice", url)
	default:
	}
}

func TestActorMerge(t *testing.T) {
	h := start(t, slowConfig())
	short := "at://did:plc:a/app.bsky.feed.post/p1"
	h.events <- stream.Message{TimeUS: us(start0), DID: "did:plc:a", RKey: "p1", Kind: stream.KindPost, SubjectURI: short, URL: "https://nyti.ms/abc"}
	h.events <- alicePosts(start0)

	if err := h.actor.Merge(context.Background(), "https://nyti.ms/abc", articleURL); err != nil {
		t.Fatal(err)
	}
	// Whichever order the actor handles them in, both posts end up under the destination.
	h.waitTop(t, func(r TopResult) bool {
		return len(r.Ranking.Week) == 1 && r.Ranking.Week[0].URL == articleURL && r.Ranking.Week[0].Week.Posts == 2
	})

	// Later likes of the short-link post are credited to the destination too.
	h.events <- likes(1, short, start0.Add(time.Minute))[0]
	h.waitTop(t, func(r TopResult) bool { return weekLikes(r, articleURL) == 1 })
}

func TestActorFlushesWhenBufferIsFull(t *testing.T) {
	cfg := slowConfig()
	cfg.MaxBuffer = 3
	h := start(t, cfg)
	h.events <- alicePosts(start0)
	for _, m := range likes(4, alicePost, start0) {
		h.events <- m
	}
	h.stop()

	batches := h.drain()
	if len(batches) != 2 || len(batches[0]) != 3 || len(batches[1]) != 2 {
		t.Fatalf("batch sizes = %v, want [3 2]", sizes(batches))
	}
}

func TestActorFlushesOnTimer(t *testing.T) {
	cfg := slowConfig()
	cfg.WriteInterval = 20 * time.Millisecond
	h := start(t, cfg)
	h.events <- alicePosts(start0)

	select {
	case b := <-h.batches:
		if len(b) != 1 {
			t.Fatalf("batch = %+v", b)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no batch after write interval")
	}
}

func TestActorShutdownSavesEverything(t *testing.T) {
	h := start(t, slowConfig())
	h.events <- alicePosts(start0)
	for _, m := range likes(10, alicePost, start0) {
		h.events <- m
	}
	// Close straight away; queued events must still be handled and saved.
	h.stop()

	total := 0
	for _, b := range h.drain() { // drain ends because Run closed batches
		total += len(b)
	}
	if total != 11 {
		t.Fatalf("saved %d events, want 11", total)
	}

	if _, err := h.actor.Top(context.Background()); !errors.Is(err, ErrStopped) {
		t.Fatalf("Top after stop: %v, want ErrStopped", err)
	}
	if err := h.actor.Merge(context.Background(), "a", "b"); !errors.Is(err, ErrStopped) {
		t.Fatalf("Merge after stop: %v, want ErrStopped", err)
	}
}

// When the writer is behind, the actor waits instead of growing its memory,
// and events back up in the events channel.
func TestActorWaitsForWriter(t *testing.T) {
	cfg := slowConfig()
	cfg.MaxBuffer = 1
	h := &harness{
		actor:   NewActor(cfg, discard),
		events:  make(chan stream.Message, 100),
		batches: make(chan []event.Event), // nobody reading yet
		done:    make(chan struct{}),
	}
	go func() { h.actor.Run(h.events, h.batches); close(h.done) }()

	h.events <- alicePosts(start0)
	for _, m := range likes(5, alicePost, start0) {
		h.events <- m
	}
	time.Sleep(50 * time.Millisecond)
	if n := len(h.events); n < 4 {
		t.Fatalf("only %d events waiting; the actor should be stuck handing over its first batch", n)
	}

	// Once the writer catches up, everything comes through.
	got := 0
	close(h.events)
	for b := range h.batches {
		got += len(b)
	}
	if got != 6 {
		t.Fatalf("got %d events, want 6", got)
	}
}

// Old posts and links are already left out of the rankings; sweeping is
// about freeing their memory, so check the sizes directly.
func TestActorSweep(t *testing.T) {
	a := NewActor(slowConfig(), discard) // not running; called directly
	old := "at://did:plc:old/app.bsky.feed.post/1"
	a.proc.Handle(stream.Message{TimeUS: us(start0), DID: "did:plc:old", RKey: "1", Kind: stream.KindPost, SubjectURI: old, URL: "https://old.com"})
	a.proc.Handle(alicePosts(start0.Add(8 * 24 * time.Hour))) // moves "now" forward 8 days

	a.sweep()

	if n := a.proc.posts.Len(); n != 1 {
		t.Errorf("post cache has %d posts, want 1", n)
	}
	if s := a.proc.ranking.Stats(); s.Links != 1 {
		t.Errorf("ranking has %d links, want 1", s.Links)
	}
}

func TestTopCancelled(t *testing.T) {
	a := NewActor(slowConfig(), discard) // never started, so it never answers
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := a.Top(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func sizes(batches [][]event.Event) []int {
	var s []int
	for _, b := range batches {
		s = append(s, len(b))
	}
	return s
}
