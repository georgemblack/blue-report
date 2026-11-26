package ranking

import (
	"fmt"
	"testing"
	"time"

	"github.com/georgemblack/blue-report/backend/internal/event"
)

const (
	nytURL   = "https://www.nytimes.com/2026/11/26/example.html"
	bbcURL   = "https://www.bbc.co.uk/news/example"
	shortURL = "https://nyti.ms/abc123"

	alicePost = "at://did:plc:alice/app.bsky.feed.post/3kabc"
	bobPost   = "at://did:plc:bob/app.bsky.feed.post/3kdef"
)

var now = time.Date(2026, 11, 26, 15, 0, 0, 0, time.UTC)

func ago(d time.Duration) time.Time {
	return now.Add(-d)
}

// find returns the entry for url in a list, or fails the test.
func find(t *testing.T, list []Entry, url string) Entry {
	t.Helper()
	for _, e := range list {
		if e.URL == url {
			return e
		}
	}
	t.Fatalf("%s not found in list of %d entries", url, len(list))
	return Entry{}
}

func absent(t *testing.T, list []Entry, url string) {
	t.Helper()
	for _, e := range list {
		if e.URL == url {
			t.Fatalf("%s unexpectedly in list: %+v", url, e)
		}
	}
}

func urls(list []Entry) []string {
	var out []string
	for _, e := range list {
		out = append(out, e.URL)
	}
	return out
}

func TestUserCountsOncePerType(t *testing.T) {
	r := New()
	r.Record(nytURL, "did:plc:bob", alicePost, event.Like, ago(30*time.Minute))
	r.Record(nytURL, "did:plc:bob", alicePost, event.Like, ago(20*time.Minute))
	r.Record(nytURL, "did:plc:bob", bobPost, event.Like, ago(10*time.Minute)) // a different post, same link

	got := find(t, r.Top(now, 10).Hour, nytURL).Hour
	if want := (Counts{Likes: 1}); got != want {
		t.Fatalf("Hour counts = %+v, want %+v", got, want)
	}
}

func TestDifferentTypesCountSeparately(t *testing.T) {
	r := New()
	r.Record(nytURL, "did:plc:bob", alicePost, event.Post, ago(time.Minute))
	r.Record(nytURL, "did:plc:bob", alicePost, event.Repost, ago(time.Minute))
	r.Record(nytURL, "did:plc:bob", alicePost, event.Like, ago(time.Minute))

	got := find(t, r.Top(now, 10).Hour, nytURL).Hour
	if want := (Counts{Posts: 1, Reposts: 1, Likes: 1}); got != want {
		t.Fatalf("Hour counts = %+v, want %+v", got, want)
	}
	if got.Score() != 21 {
		t.Fatalf("Score() = %d, want 21", got.Score())
	}
}

func TestDifferentUsersCountSeparately(t *testing.T) {
	r := New()
	for i := range 5 {
		r.Record(nytURL, fmt.Sprintf("did:plc:user%d", i), alicePost, event.Like, ago(time.Minute))
	}
	got := find(t, r.Top(now, 10).Hour, nytURL).Hour
	if want := (Counts{Likes: 5}); got != want {
		t.Fatalf("Hour counts = %+v, want %+v", got, want)
	}
}

func TestWindows(t *testing.T) {
	r := New()
	r.Record(nytURL, "did:plc:a", alicePost, event.Like, ago(30*time.Minute)) // hour, day, week
	r.Record(nytURL, "did:plc:b", alicePost, event.Like, ago(2*time.Hour))    // day, week
	r.Record(nytURL, "did:plc:c", alicePost, event.Like, ago(2*Day))          // week
	r.Record(nytURL, "did:plc:d", alicePost, event.Like, ago(8*Day))          // none

	res := r.Top(now, 10)
	e := find(t, res.Week, nytURL)
	if e.Hour.Likes != 1 || e.Day.Likes != 2 || e.Week.Likes != 3 {
		t.Fatalf("likes hour/day/week = %d/%d/%d, want 1/2/3", e.Hour.Likes, e.Day.Likes, e.Week.Likes)
	}

	// The same counts appear whichever list the link is found in.
	if find(t, res.Hour, nytURL).Week != e.Week || find(t, res.Day, nytURL).Week != e.Week {
		t.Fatal("week counts differ between lists")
	}
}

func TestWindowBoundaryIsInclusive(t *testing.T) {
	r := New()
	r.Record(nytURL, "did:plc:a", alicePost, event.Like, ago(Hour))
	r.Record(nytURL, "did:plc:b", alicePost, event.Like, ago(Hour+time.Second))

	if got := find(t, r.Top(now, 10).Hour, nytURL).Hour.Likes; got != 1 {
		t.Fatalf("hour likes = %d, want 1", got)
	}
}

func TestLinksWithNoActivityInWindowAreLeftOut(t *testing.T) {
	r := New()
	r.Record(nytURL, "did:plc:a", alicePost, event.Like, ago(time.Minute))
	r.Record(bbcURL, "did:plc:a", bobPost, event.Like, ago(3*Day))

	res := r.Top(now, 10)
	absent(t, res.Hour, bbcURL)
	absent(t, res.Day, bbcURL)
	find(t, res.Week, bbcURL)
}

// Deletes (unlikes) are ignored, so like → unlike → like is two likes from
// the same user, which counts once.
func TestLikeUnlikeLikeCountsOnce(t *testing.T) {
	r := New()
	r.Record(nytURL, "did:plc:bob", alicePost, event.Like, ago(40*time.Minute))
	r.Record(nytURL, "did:plc:bob", alicePost, event.Like, ago(5*time.Minute))

	e := find(t, r.Top(now, 10).Week, nytURL)
	if e.Hour.Likes != 1 || e.Day.Likes != 1 || e.Week.Likes != 1 {
		t.Fatalf("likes hour/day/week = %d/%d/%d, want 1/1/1", e.Hour.Likes, e.Day.Likes, e.Week.Likes)
	}
}

// v1's bug: an old like could be kept and a recent one dropped, so the user
// was missing from the hour ranking. Here the order doesn't matter.
func TestOldLikePlusRecentLikeCountsInHour(t *testing.T) {
	for _, recentFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("recentFirst=%v", recentFirst), func(t *testing.T) {
			r := New()
			old, recent := ago(3*Day), ago(10*time.Minute)
			if recentFirst {
				old, recent = recent, old
			}
			r.Record(nytURL, "did:plc:bob", alicePost, event.Like, old)
			r.Record(nytURL, "did:plc:bob", alicePost, event.Like, recent)

			e := find(t, r.Top(now, 10).Hour, nytURL)
			if e.Hour.Likes != 1 || e.Week.Likes != 1 {
				t.Fatalf("likes hour/week = %d/%d, want 1/1", e.Hour.Likes, e.Week.Likes)
			}
		})
	}
}

func TestOrderAndLimit(t *testing.T) {
	r := New()
	// link0 gets 0 likes … link4 gets 4 likes; link5 gets one post (worth 10).
	for i := range 5 {
		url := fmt.Sprintf("https://example.com/%d", i)
		r.Record(url, "did:plc:poster", alicePost, event.Repost, ago(2*Day)) // keeps every link in the week list
		for j := range i {
			r.Record(url, fmt.Sprintf("did:plc:user%d", j), alicePost, event.Like, ago(time.Minute))
		}
	}
	r.Record("https://example.com/5", "did:plc:poster", alicePost, event.Post, ago(time.Minute))

	got := urls(r.Top(now, 3).Hour)
	want := []string{"https://example.com/5", "https://example.com/4", "https://example.com/3"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("hour order = %v, want %v", got, want)
	}

	if got := r.Top(now, 0); len(got.Hour)+len(got.Day)+len(got.Week) != 0 {
		t.Fatalf("Top(n=0) returned entries: %+v", got)
	}
}

func TestTiesAreOrderedByURL(t *testing.T) {
	r := New()
	for _, url := range []string{"https://c.com", "https://a.com", "https://b.com"} {
		r.Record(url, "did:plc:a", alicePost, event.Like, ago(time.Minute))
	}
	got := urls(r.Top(now, 10).Hour)
	want := []string{"https://a.com", "https://b.com", "https://c.com"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestTopPosts(t *testing.T) {
	r := New()
	// alicePost gets 3 interactions, bobPost 1, and an old post only 8 days ago.
	r.Record(nytURL, "did:plc:alice", alicePost, event.Post, ago(time.Hour))
	r.Record(nytURL, "did:plc:x", alicePost, event.Like, ago(time.Minute))
	r.Record(nytURL, "did:plc:y", alicePost, event.Repost, ago(time.Minute))
	r.Record(nytURL, "did:plc:bob", bobPost, event.Post, ago(time.Minute))
	r.Record(nytURL, "did:plc:old", "at://did:plc:old/app.bsky.feed.post/1", event.Post, ago(8*Day))

	posts := find(t, r.Top(now, 10).Hour, nytURL).Posts
	want := []Post{{URI: alicePost, Count: 3}, {URI: bobPost, Count: 1}}
	if fmt.Sprint(posts) != fmt.Sprint(want) {
		t.Fatalf("posts = %v, want %v", posts, want)
	}
}

func TestTopPostsLimit(t *testing.T) {
	r := New()
	for i := range PostsPerLink + 5 {
		r.Record(nytURL, fmt.Sprintf("did:plc:u%d", i), fmt.Sprintf("at://did:plc:u%d/app.bsky.feed.post/1", i), event.Post, ago(time.Minute))
	}
	if got := len(find(t, r.Top(now, 10).Hour, nytURL).Posts); got != PostsPerLink {
		t.Fatalf("got %d posts, want %d", got, PostsPerLink)
	}
}

func TestTopResultIsACopy(t *testing.T) {
	r := New()
	r.Record(nytURL, "did:plc:a", alicePost, event.Like, ago(time.Minute))
	res := r.Top(now, 10)

	r.Record(nytURL, "did:plc:b", alicePost, event.Like, ago(time.Minute))
	r.Merge(nytURL, bbcURL)

	e := res.Hour[0]
	if e.URL != nytURL || e.Hour.Likes != 1 || e.Posts[0].Count != 1 {
		t.Fatalf("earlier result changed: %+v", e)
	}
}

func TestMerge(t *testing.T) {
	r := New()
	// Bob liked both the short link and its destination; he should count once.
	r.Record(shortURL, "did:plc:bob", alicePost, event.Like, ago(2*Day))
	r.Record(nytURL, "did:plc:bob", bobPost, event.Like, ago(3*Day))
	// Carol only liked the short link, recently.
	r.Record(shortURL, "did:plc:carol", alicePost, event.Like, ago(10*time.Minute))
	// Dan posted the destination directly.
	r.Record(nytURL, "did:plc:dan", bobPost, event.Post, ago(2*time.Hour))

	r.Merge(shortURL, nytURL)

	res := r.Top(now, 10)
	absent(t, res.Week, shortURL)
	e := find(t, res.Week, nytURL)
	if want := (Counts{Posts: 1, Likes: 2}); e.Week != want {
		t.Fatalf("week counts = %+v, want %+v", e.Week, want)
	}
	if want := (Counts{Likes: 1}); e.Hour != want {
		t.Fatalf("hour counts = %+v, want %+v", e.Hour, want)
	}
	wantPosts := []Post{{URI: alicePost, Count: 2}, {URI: bobPost, Count: 2}}
	if fmt.Sprint(e.Posts) != fmt.Sprint(wantPosts) {
		t.Fatalf("posts = %v, want %v", e.Posts, wantPosts)
	}
	if s := r.Stats(); s.Links != 1 {
		t.Fatalf("Links = %d after merge, want 1", s.Links)
	}
}

func TestMergeKeepsLatestTime(t *testing.T) {
	r := New()
	r.Record(nytURL, "did:plc:bob", alicePost, event.Like, ago(3*Day))
	r.Record(shortURL, "did:plc:bob", alicePost, event.Like, ago(5*time.Minute))
	r.Merge(shortURL, nytURL)

	if got := find(t, r.Top(now, 10).Hour, nytURL).Hour.Likes; got != 1 {
		t.Fatalf("hour likes = %d, want 1", got)
	}
}

func TestMergeIntoNewLink(t *testing.T) {
	r := New()
	r.Record(shortURL, "did:plc:bob", alicePost, event.Like, ago(time.Minute))
	r.Merge(shortURL, nytURL)

	res := r.Top(now, 10)
	absent(t, res.Hour, shortURL)
	if got := find(t, res.Hour, nytURL).Hour.Likes; got != 1 {
		t.Fatalf("hour likes = %d, want 1", got)
	}
}

func TestMergeNoOps(t *testing.T) {
	r := New()
	r.Record(nytURL, "did:plc:bob", alicePost, event.Like, ago(time.Minute))

	r.Merge(nytURL, nytURL)   // into itself
	r.Merge(shortURL, nytURL) // unknown source

	if s := r.Stats(); s != (Stats{Links: 1, Users: 1, Posts: 1}) {
		t.Fatalf("Stats() = %+v, want 1/1/1", s)
	}
	if got := find(t, r.Top(now, 10).Hour, nytURL).Hour.Likes; got != 1 {
		t.Fatalf("hour likes = %d, want 1", got)
	}
}

func TestSweep(t *testing.T) {
	r := New()
	// nytURL has one old and one recent user; bbcURL is entirely old.
	r.Record(nytURL, "did:plc:old", "at://did:plc:old/app.bsky.feed.post/1", event.Post, ago(8*Day))
	r.Record(nytURL, "did:plc:new", alicePost, event.Like, ago(time.Hour))
	r.Record(bbcURL, "did:plc:old", bobPost, event.Post, ago(8*Day))

	r.Sweep(now.Add(-Week))

	if s := r.Stats(); s != (Stats{Links: 1, Users: 1, Posts: 1}) {
		t.Fatalf("Stats() after sweep = %+v, want 1/1/1", s)
	}
	e := find(t, r.Top(now, 10).Week, nytURL)
	if want := (Counts{Likes: 1}); e.Week != want {
		t.Fatalf("week counts = %+v, want %+v", e.Week, want)
	}
}

func TestSweepKeepsEntriesAtCutoff(t *testing.T) {
	r := New()
	r.Record(nytURL, "did:plc:a", alicePost, event.Like, ago(Week))
	r.Sweep(now.Add(-Week))
	if s := r.Stats(); s.Users != 1 {
		t.Fatalf("Users = %d, want 1", s.Users)
	}
}

func TestPopular(t *testing.T) {
	r := New()
	for i := range 3 {
		r.Record(nytURL, fmt.Sprintf("did:plc:u%d", i), alicePost, event.Like, ago(time.Minute))
	}
	r.Record(bbcURL, "did:plc:u0", bobPost, event.Like, ago(time.Minute))
	r.Record(bbcURL, "did:plc:u0", bobPost, event.Repost, ago(time.Minute))

	if got := r.Popular(3); fmt.Sprint(got) != fmt.Sprint([]string{nytURL}) {
		t.Fatalf("Popular(3) = %v", got)
	}
	if got := r.Popular(2); fmt.Sprint(got) != fmt.Sprint([]string{bbcURL, nytURL}) {
		t.Fatalf("Popular(2) = %v", got)
	}
}

func TestUserKeyKeepsTypesApart(t *testing.T) {
	for _, did := range []string{"did:plc:a", "did:plc:b", "did:web:example.com"} {
		seen := map[uint64]bool{}
		for _, typ := range []event.Type{event.Post, event.Repost, event.Like} {
			k := userKey(did, typ)
			if keyType(k) != typ {
				t.Fatalf("keyType(userKey(%q, %d)) = %d", did, typ, keyType(k))
			}
			if seen[k] {
				t.Fatalf("duplicate key for %q", did)
			}
			seen[k] = true
		}
	}
}

// BenchmarkTop measures one Top call over a ranking roughly the size we
// expect in production. Adjust the sizes once we've measured real data.
func BenchmarkTop(b *testing.B) {
	const links = 200_000
	r := New()
	for i := range links {
		url := fmt.Sprintf("https://example.com/%d", i)
		users := 1 + i%10
		for j := range users {
			r.Record(url, fmt.Sprintf("did:plc:%d", j), alicePost, event.Type(j%3), ago(time.Duration(j)*time.Hour))
		}
	}
	b.ResetTimer()
	for b.Loop() {
		r.Top(now, 10)
	}
}
