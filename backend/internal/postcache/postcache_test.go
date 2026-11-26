package postcache

import (
	"testing"
	"time"
)

const (
	alicePost = "at://did:plc:alice/app.bsky.feed.post/3kabc"
	bobPost   = "at://did:plc:bob/app.bsky.feed.post/3kdef"
	nytURL    = "https://www.nytimes.com/2026/11/26/example.html"
	bbcURL    = "https://www.bbc.co.uk/news/example"
)

var start = time.Date(2026, 11, 26, 15, 0, 0, 0, time.UTC)

func newCache() *Cache {
	return New(12 * time.Hour)
}

func mustGet(t *testing.T, c *Cache, uri string, now time.Time, want string) {
	t.Helper()
	got, ok := c.Get(uri, now)
	if !ok {
		t.Fatalf("Get(%q) at %v: not found, want %q", uri, now, want)
	}
	if got != want {
		t.Fatalf("Get(%q) = %q, want %q", uri, got, want)
	}
}

func mustMiss(t *testing.T, c *Cache, uri string, now time.Time) {
	t.Helper()
	if got, ok := c.Get(uri, now); ok {
		t.Fatalf("Get(%q) at %v = %q, want not found", uri, now, got)
	}
}

func TestPutAndGet(t *testing.T) {
	c := newCache()
	c.Put(alicePost, nytURL, start)
	c.Put(bobPost, bbcURL, start)

	mustGet(t, c, alicePost, start.Add(time.Minute), nytURL)
	mustGet(t, c, bobPost, start.Add(time.Minute), bbcURL)
	mustMiss(t, c, "at://did:plc:carol/app.bsky.feed.post/3kxyz", start)

	if c.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", c.Len())
	}
}

func TestExpiresAfterTTL(t *testing.T) {
	c := newCache()
	c.Put(alicePost, nytURL, start)

	// Exactly at the TTL still counts as cached.
	mustGet(t, c, alicePost, start.Add(12*time.Hour), nytURL)

	c = newCache()
	c.Put(alicePost, nytURL, start)
	mustMiss(t, c, alicePost, start.Add(12*time.Hour+time.Second))

	// An expired post is removed when looked up.
	if c.Len() != 0 {
		t.Fatalf("Len() = %d after expired lookup, want 0", c.Len())
	}
}

func TestGetKeepsActivePostsCached(t *testing.T) {
	c := newCache()
	c.Put(alicePost, nytURL, start)

	// A like every 10 hours keeps the post alive well past the first 12 hours.
	for i := 1; i <= 5; i++ {
		mustGet(t, c, alicePost, start.Add(time.Duration(i)*10*time.Hour), nytURL)
	}

	// Then 13 hours of silence.
	mustMiss(t, c, alicePost, start.Add(63*time.Hour))
}

func TestGetWithOlderTimeDoesNotMoveBackwards(t *testing.T) {
	c := newCache()
	c.Put(alicePost, nytURL, start.Add(10*time.Hour))

	// A lookup with an earlier time is still found and doesn't change the last-active time.
	mustGet(t, c, alicePost, start, nytURL)
	mustGet(t, c, alicePost, start.Add(22*time.Hour), nytURL)
}

func TestPutKeepsNewestTime(t *testing.T) {
	c := newCache()

	// During a rebuild, rows can arrive in any order.
	c.Put(alicePost, nytURL, start.Add(6*time.Hour))
	c.Put(alicePost, nytURL, start)

	// Still cached 12 hours after the newest time, not the last one written.
	mustGet(t, c, alicePost, start.Add(18*time.Hour), nytURL)
}

func TestSweep(t *testing.T) {
	c := newCache()
	c.Put(alicePost, nytURL, start)
	c.Put(bobPost, bbcURL, start.Add(2*time.Hour))

	if removed := c.Sweep(start.Add(time.Hour)); removed != 1 {
		t.Fatalf("Sweep removed %d, want 1", removed)
	}
	if c.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", c.Len())
	}
	mustMiss(t, c, alicePost, start.Add(2*time.Hour))
	mustGet(t, c, bobPost, start.Add(2*time.Hour), bbcURL)

	// Entries exactly at the cutoff are kept.
	if removed := c.Sweep(start.Add(2 * time.Hour)); removed != 0 {
		t.Fatalf("Sweep at the exact time removed %d, want 0", removed)
	}
}
