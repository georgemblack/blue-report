// Package postcache remembers which link each recently active post shared.
// When someone likes, reposts or quotes a post, the cache tells us which link
// to credit.
//
// A post stays cached while it keeps getting activity. Once it has had none
// for the cache's TTL (12 hours in production), it's treated as gone.
//
// Not safe for use by more than one goroutine; the state actor owns it.
package postcache

import (
	"time"

	"github.com/georgemblack/blue-report/backend/internal/event"
	"github.com/georgemblack/blue-report/backend/internal/hash"
)

type Cache struct {
	entries map[uint64]entry
	ttl     uint32 // seconds
}

type entry struct {
	url      string
	lastSeen uint32 // seconds since the Unix epoch
}

func New(ttl time.Duration) *Cache {
	return &Cache{
		entries: make(map[uint64]entry),
		ttl:     uint32(ttl / time.Second),
	}
}

// Put records that a post shared a link, and was active at the given time.
// If the post is already cached, its last-active time only moves forward, so
// calls can arrive in any order (as they do during the startup rebuild).
func (c *Cache) Put(postURI, url string, at time.Time) {
	key := hash.String(postURI)
	t := event.Seconds(at)
	if e, ok := c.entries[key]; ok && e.lastSeen > t {
		t = e.lastSeen
	}
	c.entries[key] = entry{url: url, lastSeen: t}
}

// Get returns the link a post shared. A post that's missing, or inactive for
// longer than the TTL, is reported as not found. A found post is marked as
// active at the given time, which keeps busy posts in the cache.
func (c *Cache) Get(postURI string, now time.Time) (string, bool) {
	key := hash.String(postURI)
	e, ok := c.entries[key]
	if !ok {
		return "", false
	}
	t := event.Seconds(now)
	if t > e.lastSeen && t-e.lastSeen > c.ttl {
		delete(c.entries, key)
		return "", false
	}
	if t > e.lastSeen {
		e.lastSeen = t
		c.entries[key] = e
	}
	return e.url, true
}

// Sweep removes posts that haven't been active since the cutoff, and returns
// how many were removed. Normally the cutoff is now minus the TTL; a more
// recent cutoff can be used to shrink the cache if it grows too large.
func (c *Cache) Sweep(cutoff time.Time) int {
	t := event.Seconds(cutoff)
	removed := 0
	for key, e := range c.entries {
		if e.lastSeen < t {
			delete(c.entries, key)
			removed++
		}
	}
	return removed
}

// Len returns the number of cached posts, including any that have expired
// but haven't been swept yet.
func (c *Cache) Len() int {
	return len(c.entries)
}
