package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/georgemblack/blue-report/backend/internal/event"
	"github.com/georgemblack/blue-report/backend/internal/jetstream"
	"github.com/georgemblack/blue-report/backend/internal/ranking"
)

const (
	alicePost  = "at://did:plc:alice/app.bsky.feed.post/3kabc"
	articleURL = "https://www.nytimes.com/2026/11/26/example.html"
)

var start = time.Date(2026, 11, 26, 15, 0, 0, 0, time.UTC)

func us(t time.Time) int64 { return t.UnixMicro() }

// alicePosts is Alice sharing articleURL (with a tracking parameter) at the given time.
func alicePosts(at time.Time) jetstream.Message {
	return jetstream.Message{
		TimeUS: us(at), DID: "did:plc:alice", RKey: "3kabc", Kind: jetstream.KindPost,
		SubjectURI: alicePost, URL: articleURL + "?smid=share",
	}
}

func interaction(kind jetstream.Kind, did, rkey, subject string, at time.Time) jetstream.Message {
	return jetstream.Message{TimeUS: us(at), DID: did, RKey: rkey, Kind: kind, SubjectURI: subject}
}

func mustHandle(t *testing.T, p *Processor, m jetstream.Message, want bool) {
	t.Helper()
	if got := p.Handle(m); got != want {
		t.Fatalf("Handle(%+v) = %v, want %v", m, got, want)
	}
}

func weekCounts(t *testing.T, p *Processor, url string) ranking.Counts {
	t.Helper()
	for _, e := range p.ranking.Top(p.Newest(), 100).Week {
		if e.URL == url {
			return e.Week
		}
	}
	t.Fatalf("%s not ranked", url)
	return ranking.Counts{}
}

func TestPostAndInteractions(t *testing.T) {
	p := NewProcessor()
	mustHandle(t, p, alicePosts(start), true)
	mustHandle(t, p, interaction(jetstream.KindLike, "did:plc:bob", "l1", alicePost, start.Add(time.Minute)), true)
	mustHandle(t, p, interaction(jetstream.KindRepost, "did:plc:carol", "r1", alicePost, start.Add(2*time.Minute)), true)
	mustHandle(t, p, interaction(jetstream.KindQuote, "did:plc:dan", "q1", alicePost, start.Add(3*time.Minute)), true)

	if got, want := weekCounts(t, p, articleURL), (ranking.Counts{Posts: 2, Reposts: 1, Likes: 1}); got != want {
		t.Fatalf("counts = %+v, want %+v", got, want)
	}

	got := p.TakeBuffer()
	want := []event.Event{
		{TimeUS: us(start), Type: event.Post, DID: "did:plc:alice", RKey: "3kabc", SubjectURI: alicePost, URL: articleURL},
		{TimeUS: us(start.Add(time.Minute)), Type: event.Like, DID: "did:plc:bob", RKey: "l1", SubjectURI: alicePost, URL: articleURL},
		{TimeUS: us(start.Add(2 * time.Minute)), Type: event.Repost, DID: "did:plc:carol", RKey: "r1", SubjectURI: alicePost, URL: articleURL},
		{TimeUS: us(start.Add(3 * time.Minute)), Type: event.Post, DID: "did:plc:dan", RKey: "q1", SubjectURI: alicePost, URL: articleURL},
	}
	if len(got) != len(want) {
		t.Fatalf("buffer has %d events, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("event %d:\n got  %+v\n want %+v", i, got[i], want[i])
		}
	}

	if s := p.Stats(); s != (Stats{Posts: 1, Quotes: 1, Reposts: 1, Likes: 1}) {
		t.Fatalf("Stats() = %+v", s)
	}
}

func TestSkipped(t *testing.T) {
	p := NewProcessor()

	// A like of a post we never saw.
	mustHandle(t, p, interaction(jetstream.KindLike, "did:plc:bob", "l1", alicePost, start), false)
	// A post without a link.
	mustHandle(t, p, jetstream.Message{TimeUS: us(start), DID: "did:plc:a", RKey: "p1", Kind: jetstream.KindPost, SubjectURI: "at://did:plc:a/app.bsky.feed.post/p1"}, false)
	// A post with an ignored link, and then a like of it.
	ignored := "at://did:plc:a/app.bsky.feed.post/p2"
	mustHandle(t, p, jetstream.Message{TimeUS: us(start), DID: "did:plc:a", RKey: "p2", Kind: jetstream.KindPost, SubjectURI: ignored, URL: "https://media.tenor.com/x.gif"}, false)
	mustHandle(t, p, interaction(jetstream.KindLike, "did:plc:bob", "l2", ignored, start), false)

	if s := p.Stats(); s != (Stats{NoLink: 1, Ignored: 1, NotCached: 2}) {
		t.Fatalf("Stats() = %+v", s)
	}
	if p.BufferLen() != 0 || p.ranking.Stats().Links != 0 {
		t.Fatalf("skipped events left state behind: buffer=%d links=%d", p.BufferLen(), p.ranking.Stats().Links)
	}
}

func TestCacheExpiry(t *testing.T) {
	p := NewProcessor()
	mustHandle(t, p, alicePosts(start), true)

	// Activity every 11 hours keeps the post cached…
	mustHandle(t, p, interaction(jetstream.KindLike, "did:plc:b", "l1", alicePost, start.Add(11*time.Hour)), true)
	mustHandle(t, p, interaction(jetstream.KindLike, "did:plc:c", "l2", alicePost, start.Add(22*time.Hour)), true)
	// …until 12 hours pass with none.
	mustHandle(t, p, interaction(jetstream.KindLike, "did:plc:d", "l3", alicePost, start.Add(34*time.Hour+time.Second)), false)
}

func TestTranslation(t *testing.T) {
	p := NewProcessor()
	p.translations.Add("https://nyti.ms/abc", articleURL)

	short := "at://did:plc:a/app.bsky.feed.post/p1"
	mustHandle(t, p, jetstream.Message{TimeUS: us(start), DID: "did:plc:a", RKey: "p1", Kind: jetstream.KindPost, SubjectURI: short, URL: "https://nyti.ms/abc"}, true)
	mustHandle(t, p, alicePosts(start), true)

	// Both posts count toward the final link.
	if got := weekCounts(t, p, articleURL); got.Posts != 2 {
		t.Fatalf("posts = %d, want 2", got.Posts)
	}
	// The stored event keeps the short link, so redirects can be re-applied later.
	if b := p.TakeBuffer(); b[0].URL != "https://nyti.ms/abc" {
		t.Fatalf("stored URL = %q, want the short link", b[0].URL)
	}
}

// A translation learned after a post is cached applies to later likes.
func TestTranslationAddedLater(t *testing.T) {
	p := NewProcessor()
	short := "at://did:plc:a/app.bsky.feed.post/p1"
	mustHandle(t, p, jetstream.Message{TimeUS: us(start), DID: "did:plc:a", RKey: "p1", Kind: jetstream.KindPost, SubjectURI: short, URL: "https://nyti.ms/abc"}, true)

	p.translations.Add("https://nyti.ms/abc", articleURL)
	mustHandle(t, p, interaction(jetstream.KindLike, "did:plc:b", "l1", short, start.Add(time.Minute)), true)

	if got := weekCounts(t, p, articleURL); got.Likes != 1 {
		t.Fatalf("likes = %d, want 1", got.Likes)
	}
}

func TestTakeBufferStartsNewBuffer(t *testing.T) {
	p := NewProcessor()
	mustHandle(t, p, alicePosts(start), true)
	first := p.TakeBuffer()
	mustHandle(t, p, interaction(jetstream.KindLike, "did:plc:b", "l1", alicePost, start), true)

	if len(first) != 1 || p.BufferLen() != 1 {
		t.Fatalf("first=%d current=%d, want 1 and 1", len(first), p.BufferLen())
	}
	if first[0].Type != event.Post {
		t.Fatal("earlier buffer was changed")
	}
}

func TestNewest(t *testing.T) {
	p := NewProcessor()
	if !p.Newest().IsZero() {
		t.Fatal("Newest() should be zero before any events")
	}
	p.Handle(alicePosts(start.Add(time.Hour)))
	p.Handle(interaction(jetstream.KindLike, "did:plc:b", "l1", "at://unknown", start)) // older, and skipped
	if !p.Newest().Equal(start.Add(time.Hour)) {
		t.Fatalf("Newest() = %v", p.Newest())
	}
}

// Runs real Jetstream messages through the parser and the processor.
func TestWithJetstreamFixtures(t *testing.T) {
	parser := jetstream.NewParser()
	p := NewProcessor()
	for _, name := range []string{"post-embed-only.json", "post-facet-only.json", "post-no-url.json", "post-quote.json", "like.json"} {
		data, err := os.ReadFile(filepath.Join("..", "jetstream", "testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		m, ok, err := parser.Parse(data)
		if err != nil || !ok {
			t.Fatalf("%s: ok=%v err=%v", name, ok, err)
		}
		p.Handle(m)
	}

	// Two posts share the same link. The quote and like are of a post that
	// isn't in the fixtures, so they're skipped.
	if s := p.Stats(); s != (Stats{Posts: 2, NoLink: 1, NotCached: 2}) {
		t.Fatalf("Stats() = %+v", s)
	}
	if got := weekCounts(t, p, "https://tylervigen.com/the-mystery-of-the-bloomfield-bridge"); got.Posts != 1 {
		t.Fatalf("posts = %d, want 1 (same user posted twice)", got.Posts)
	}
}
