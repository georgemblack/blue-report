// Package ranking keeps track of who interacted with each link and when, and
// ranks links over the last hour, day and week.
//
// Each user counts at most once per link and interaction type, no matter how
// many times they post, repost or like it. For each (link, user, type) we keep
// only the user's latest time, so a user counts in a window if they did that
// thing at least once within it. Because only the latest time is kept, events
// can be recorded in any order and duplicates are harmless.
//
// Not safe for use by more than one goroutine; the state actor owns it.
package ranking

import (
	"cmp"
	"slices"
	"time"

	"github.com/georgemblack/blue-report/backend/internal/event"
	"github.com/georgemblack/blue-report/backend/internal/hash"
)

// PostsPerLink is how many of a link's most active posts Top returns. These
// are the candidates for a link's recommended posts.
const PostsPerLink = 20

// The ranking windows.
const (
	Hour = time.Hour
	Day  = 24 * time.Hour
	Week = 7 * 24 * time.Hour
)

type Ranking struct {
	links map[uint64]*link // keyed by hash of the final URL
}

type link struct {
	url   string
	users map[uint64]uint32   // userKey → latest time (seconds since the Unix epoch)
	posts map[uint64]postStat // hash of the post's address → activity on that post
}

type postStat struct {
	uri      string
	count    uint32
	lastSeen uint32
}

// userKey packs a user and an interaction type into one map key. The lowest
// two bits of the user's hash are replaced by the type.
func userKey(did string, typ event.Type) uint64 {
	return hash.String(did)&^3 | uint64(typ)
}

func keyType(key uint64) event.Type {
	return event.Type(key & 3)
}

func New() *Ranking {
	return &Ranking{links: make(map[uint64]*link)}
}

// Record notes that a user interacted with a link at the given time.
// subjectURI is the address of the post that shared the link: the post
// itself for a post, or the post being quoted, reposted or liked.
func (r *Ranking) Record(url, did, subjectURI string, typ event.Type, at time.Time) {
	t := event.Seconds(at)
	l := r.getOrCreate(url)

	key := userKey(did, typ)
	if t > l.users[key] {
		l.users[key] = t
	}

	postKey := hash.String(subjectURI)
	p := l.posts[postKey]
	p.uri = subjectURI
	p.count++
	p.lastSeen = max(p.lastSeen, t)
	l.posts[postKey] = p
}

// Merge folds everything recorded for src into dst, then removes src. It's
// used when we learn src redirects to dst. A user who interacted with both
// ends up counted once, with their latest time.
func (r *Ranking) Merge(src, dst string) {
	srcKey := hash.String(src)
	if srcKey == hash.String(dst) {
		return
	}
	s, ok := r.links[srcKey]
	if !ok {
		return
	}
	d := r.getOrCreate(dst)

	for key, t := range s.users {
		if t > d.users[key] {
			d.users[key] = t
		}
	}
	for key, sp := range s.posts {
		dp, ok := d.posts[key]
		if !ok {
			d.posts[key] = sp
			continue
		}
		dp.count += sp.count
		dp.lastSeen = max(dp.lastSeen, sp.lastSeen)
		d.posts[key] = dp
	}
	delete(r.links, srcKey)
}

// Remove forgets everything recorded for a link. It's used when we learn a
// link redirects somewhere on the ignore list.
func (r *Ranking) Remove(url string) {
	delete(r.links, hash.String(url))
}

// Sweep forgets interactions older than the cutoff (normally now minus a
// week), and removes links with nothing left.
func (r *Ranking) Sweep(cutoff time.Time) {
	c := event.Seconds(cutoff)
	for linkKey, l := range r.links {
		for key, t := range l.users {
			if t < c {
				delete(l.users, key)
			}
		}
		for key, p := range l.posts {
			if p.lastSeen < c {
				delete(l.posts, key)
			}
		}
		if len(l.users) == 0 && len(l.posts) == 0 {
			delete(r.links, linkKey)
		}
	}
}

// Popular returns the URLs of links with at least minUsers user entries
// (one entry per user and interaction type). These are worth checking for
// redirects, since a short link with many interactions is likely to matter.
func (r *Ranking) Popular(minUsers int) []string {
	var urls []string
	for _, l := range r.links {
		if len(l.users) >= minUsers {
			urls = append(urls, l.url)
		}
	}
	slices.Sort(urls)
	return urls
}

// Stats describes the size of the ranking state, for logging.
type Stats struct {
	Links int // number of links
	Users int // user entries across all links
	Posts int // post entries across all links
}

func (r *Ranking) Stats() Stats {
	s := Stats{Links: len(r.links)}
	for _, l := range r.links {
		s.Users += len(l.users)
		s.Posts += len(l.posts)
	}
	return s
}

// Counts are the number of distinct users who posted, reposted and liked a
// link within a window.
type Counts struct {
	Posts   int
	Reposts int
	Likes   int
}

// Score ranks links. Posts and reposts are worth ten likes (see RANKING.md).
func (c Counts) Score() int {
	return 10*c.Posts + 10*c.Reposts + c.Likes
}

func (c *Counts) add(typ event.Type) {
	switch typ {
	case event.Post:
		c.Posts++
	case event.Repost:
		c.Reposts++
	case event.Like:
		c.Likes++
	}
}

// Entry is one ranked link. It includes counts for every window, not only the
// window it was ranked in, since the published data shows week counts in all
// three lists.
type Entry struct {
	URL   string
	Hour  Counts
	Day   Counts
	Week  Counts
	Posts []Post // most active posts sharing this link, busiest first
}

// Post is a post that shared a link, with the number of interactions recorded
// for it. Posts with no activity in the last week are left out.
type Post struct {
	URI   string
	Count int
}

// Result holds the top links in each window, best first.
type Result struct {
	Hour []Entry
	Day  []Entry
	Week []Entry
}

// Top returns the n best links in each window ending at now. Links with no
// activity in a window are left out of that window's list. The result is a
// copy, and is safe to hand to other goroutines.
//
// "now" should be the newest event time seen rather than the clock, so the
// windows stay correct while catching up after a restart.
func (r *Ranking) Top(now time.Time, n int) Result {
	hourStart := event.Seconds(now.Add(-Hour))
	dayStart := event.Seconds(now.Add(-Day))
	weekStart := event.Seconds(now.Add(-Week))

	hour := newTopN(n, func(c candidate) Counts { return c.hour })
	day := newTopN(n, func(c candidate) Counts { return c.day })
	week := newTopN(n, func(c candidate) Counts { return c.week })

	// One pass over every link, counting all three windows together.
	for _, l := range r.links {
		c := candidate{link: l}
		for key, t := range l.users {
			if t < weekStart {
				continue
			}
			typ := keyType(key)
			c.week.add(typ)
			if t >= dayStart {
				c.day.add(typ)
			}
			if t >= hourStart {
				c.hour.add(typ)
			}
		}
		hour.offer(c)
		day.offer(c)
		week.offer(c)
	}

	return Result{
		Hour: hour.entries(weekStart),
		Day:  day.entries(weekStart),
		Week: week.entries(weekStart),
	}
}

type candidate struct {
	link            *link
	hour, day, week Counts
}

// topN keeps the best n candidates seen so far for one window, sorted best
// first. n is small (about 10), so a sorted slice is simpler than a heap and
// just as fast.
type topN struct {
	n      int
	counts func(candidate) Counts
	items  []candidate
}

func newTopN(n int, counts func(candidate) Counts) *topN {
	return &topN{n: n, counts: counts, items: make([]candidate, 0, n+1)}
}

// compare orders candidates best first: by score, then by URL so the order
// is stable when scores tie.
func (t *topN) compare(a, b candidate) int {
	if c := cmp.Compare(t.counts(b).Score(), t.counts(a).Score()); c != 0 {
		return c
	}
	return cmp.Compare(a.link.url, b.link.url)
}

func (t *topN) offer(c candidate) {
	if t.n <= 0 || t.counts(c).Score() == 0 {
		return
	}
	if len(t.items) == t.n && t.compare(c, t.items[len(t.items)-1]) >= 0 {
		return
	}
	i, _ := slices.BinarySearchFunc(t.items, c, t.compare)
	t.items = slices.Insert(t.items, i, c)
	if len(t.items) > t.n {
		t.items = t.items[:t.n]
	}
}

func (t *topN) entries(weekStart uint32) []Entry {
	entries := make([]Entry, 0, len(t.items))
	for _, c := range t.items {
		entries = append(entries, Entry{
			URL:   c.link.url,
			Hour:  c.hour,
			Day:   c.day,
			Week:  c.week,
			Posts: topPosts(c.link, weekStart),
		})
	}
	return entries
}

// topPosts returns a link's busiest posts within the week, busiest first.
func topPosts(l *link, weekStart uint32) []Post {
	posts := make([]Post, 0, min(len(l.posts), PostsPerLink))
	for _, p := range l.posts {
		if p.lastSeen < weekStart {
			continue
		}
		posts = append(posts, Post{URI: p.uri, Count: int(p.count)})
	}
	slices.SortFunc(posts, func(a, b Post) int {
		if c := cmp.Compare(b.Count, a.Count); c != 0 {
			return c
		}
		return cmp.Compare(a.URI, b.URI)
	})
	if len(posts) > PostsPerLink {
		posts = posts[:PostsPerLink]
	}
	return posts
}

func (r *Ranking) getOrCreate(url string) *link {
	key := hash.String(url)
	l, ok := r.links[key]
	if !ok {
		l = &link{
			url:   url,
			users: make(map[uint64]uint32),
			posts: make(map[uint64]postStat),
		}
		r.links[key] = l
	}
	return l
}
