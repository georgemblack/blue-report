// Package state holds everything the state actor owns: the post cache, the
// ranking, the translations and the buffer of events waiting to be written.
package state

import (
	"time"

	"github.com/georgemblack/blue-report/backend/internal/event"
	"github.com/georgemblack/blue-report/backend/internal/postcache"
	"github.com/georgemblack/blue-report/backend/internal/ranking"
	"github.com/georgemblack/blue-report/backend/internal/stream"
	"github.com/georgemblack/blue-report/backend/internal/translations"
	"github.com/georgemblack/blue-report/backend/internal/urltools"
)

// PostTTL is how long a post stays cached without any activity.
const PostTTL = 12 * time.Hour

// Processor applies each incoming interaction to the state. It does no
// network or disk work, so it can run inside the state actor's loop.
//
// Not safe for use by more than one goroutine.
type Processor struct {
	posts        *postcache.Cache
	ranking      *ranking.Ranking
	translations *translations.Translations
	buffer       []event.Event
	newestUS     int64 // newest event time seen, in microseconds
	stats        Stats
}

// Stats counts what happened to incoming interactions since startup.
type Stats struct {
	Posts, Quotes, Reposts, Likes int // recorded, by kind

	NoLink    int // posts without a link
	Ignored   int // links on the ignore list, or that redirect to one
	NotCached int // quotes, reposts and likes of posts not in the cache
}

func NewProcessor() *Processor {
	return &Processor{
		posts:        postcache.New(PostTTL),
		ranking:      ranking.New(),
		translations: translations.New(),
	}
}

// Handle applies one interaction. It returns true if the interaction was
// recorded, or false if it was skipped (see Stats for the reasons).
func (p *Processor) Handle(m stream.Message) bool {
	at := time.UnixMicro(m.TimeUS)
	p.newestUS = max(p.newestUS, m.TimeUS)

	// Work out which link this interaction is about, as cleaned when the
	// post was first seen. That's the version that's stored.
	var cleaned string
	if m.Kind == stream.KindPost {
		if m.URL == "" {
			p.stats.NoLink++
			return false
		}
		cleaned = urltools.Clean(m.URL)
	} else {
		url, ok := p.posts.Get(m.SubjectURI, at)
		if !ok {
			p.stats.NotCached++
			return false
		}
		cleaned = url
	}

	// Rank under the link it finally redirects to.
	final, ok := p.translations.Final(cleaned)
	if !ok {
		p.stats.Ignored++
		return false
	}

	// Only a post's own link is cached. Quotes are credited to the quoted
	// post's link, but likes of a quote aren't counted (as in v1).
	if m.Kind == stream.KindPost {
		p.posts.Put(m.SubjectURI, cleaned, at)
	}

	typ := eventType(m.Kind)
	p.ranking.Record(final, m.DID, m.SubjectURI, typ, at)
	p.buffer = append(p.buffer, event.Event{
		TimeUS:     m.TimeUS,
		Type:       typ,
		DID:        m.DID,
		RKey:       m.RKey,
		SubjectURI: m.SubjectURI,
		URL:        cleaned,
	})
	p.count(m.Kind)
	return true
}

// TakeBuffer returns the events recorded since the last call, and starts a
// new buffer. The caller owns the returned slice.
func (p *Processor) TakeBuffer() []event.Event {
	b := p.buffer
	p.buffer = nil
	return b
}

// BufferLen returns the number of events waiting to be written.
func (p *Processor) BufferLen() int {
	return len(p.buffer)
}

// Newest returns the time of the newest event seen, or the zero time if
// there hasn't been one. It's used as "now" for the ranking windows.
func (p *Processor) Newest() time.Time {
	if p.newestUS == 0 {
		return time.Time{}
	}
	return time.UnixMicro(p.newestUS)
}

// Merge records that source redirects to destination, and moves everything
// ranked under source to where it now ends up. If that's a link on the
// ignore list, source is dropped from the ranking instead.
func (p *Processor) Merge(source, destination string) {
	p.translations.Add(source, destination)
	final, ok := p.translations.Final(source)
	if !ok {
		p.ranking.Remove(source)
		return
	}
	p.ranking.Merge(source, final)
}

func (p *Processor) Stats() Stats {
	return p.stats
}

func (s Stats) recorded() int { return s.Posts + s.Quotes + s.Reposts + s.Likes }
func (s Stats) skipped() int  { return s.NoLink + s.Ignored + s.NotCached }

// sub returns the change in counts since an earlier Stats.
func (s Stats) sub(earlier Stats) Stats {
	return Stats{
		Posts: s.Posts - earlier.Posts, Quotes: s.Quotes - earlier.Quotes,
		Reposts: s.Reposts - earlier.Reposts, Likes: s.Likes - earlier.Likes,
		NoLink: s.NoLink - earlier.NoLink, Ignored: s.Ignored - earlier.Ignored,
		NotCached: s.NotCached - earlier.NotCached,
	}
}

func (p *Processor) count(k stream.Kind) {
	switch k {
	case stream.KindPost:
		p.stats.Posts++
	case stream.KindQuote:
		p.stats.Quotes++
	case stream.KindRepost:
		p.stats.Reposts++
	case stream.KindLike:
		p.stats.Likes++
	}
}

func eventType(k stream.Kind) event.Type {
	switch k {
	case stream.KindRepost:
		return event.Repost
	case stream.KindLike:
		return event.Like
	default:
		return event.Post // posts and quotes
	}
}
