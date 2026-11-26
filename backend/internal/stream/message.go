// Package stream reads Bluesky's Jetstream v2 and turns its events into the
// few kinds of interaction we care about: posts, quote posts, reposts and
// likes. The connection itself is handled by Bluesky's official client
// library (github.com/bluesky-social/jetstream).
package stream

import (
	"fmt"
	"slices"

	"github.com/bluesky-social/jetstream"
)

// Kind is the kind of interaction a message describes.
type Kind uint8

const (
	KindPost   Kind = iota // a post that isn't a quote; it may share a link
	KindQuote              // a post quoting another post
	KindRepost             // a repost of another post
	KindLike               // a like of another post
)

func (k Kind) String() string {
	switch k {
	case KindPost:
		return "post"
	case KindQuote:
		return "quote"
	case KindRepost:
		return "repost"
	case KindLike:
		return "like"
	}
	return fmt.Sprintf("Kind(%d)", uint8(k))
}

// Message is a parsed interaction.
type Message struct {
	Seq    uint64 // Jetstream's sequence number for the event
	TimeUS int64  // when Jetstream saw it, in microseconds since the Unix epoch
	DID    string // the user who made it
	RKey   string // the record key of the user's own post, repost or like
	Kind   Kind

	// SubjectURI is the address of the post this is about: the post itself
	// for a post, or the post being quoted, reposted or liked.
	SubjectURI string

	// URL is the link shared by a post, exactly as written (not cleaned).
	// It's empty for other kinds, and for posts with no link.
	URL string
}

// Collections are the record types to ask Jetstream for.
var Collections = []string{
	"app.bsky.feed.post",
	"app.bsky.feed.repost",
	"app.bsky.feed.like",
}

// FromEvent returns the interaction in a Jetstream event. ok is false when
// the event isn't one we count: anything other than a newly created post,
// repost or like, or a post not marked as English.
func FromEvent(ev jetstream.Event) (msg Message, ok bool) {
	c := ev.Commit
	if ev.Kind != jetstream.KindCommit || c == nil || c.Operation != jetstream.OpCreate {
		return Message{}, false
	}
	if ev.DID == "" || c.Rkey == "" {
		return Message{}, false
	}

	var kind Kind
	switch c.Collection {
	case "app.bsky.feed.post":
		kind = KindPost
	case "app.bsky.feed.repost":
		kind = KindRepost
	case "app.bsky.feed.like":
		kind = KindLike
	default:
		return Message{}, false
	}

	// WitnessedAtUS is the time Jetstream resumes from, so it's the one we
	// store. Older servers may not send it.
	t := ev.WitnessedAtUS
	if t == 0 {
		t = ev.TimeUS
	}
	msg = Message{Seq: ev.Seq, TimeUS: t, DID: ev.DID, RKey: c.Rkey, Kind: kind}
	record := c.Record

	if kind != KindPost {
		subject := str(obj(record, "subject"), "uri")
		if subject == "" {
			return Message{}, false
		}
		msg.SubjectURI = subject
		return msg, true
	}

	if !isEnglish(record) {
		return Message{}, false
	}
	if quoted := quotedURI(obj(record, "embed")); quoted != "" {
		msg.Kind = KindQuote
		msg.SubjectURI = quoted
		return msg, true
	}
	msg.SubjectURI = fmt.Sprintf("at://%s/app.bsky.feed.post/%s", ev.DID, c.Rkey)
	msg.URL = link(record)
	return msg, true
}

// Records arrive as generic JSON-like maps. These helpers read fields from
// them, treating anything missing or of the wrong type as empty.

func obj(m map[string]any, key string) map[string]any {
	v, _ := m[key].(map[string]any)
	return v
}

func str(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func list(m map[string]any, key string) []any {
	v, _ := m[key].([]any)
	return v
}

func isEnglish(record map[string]any) bool {
	return slices.ContainsFunc(list(record, "langs"), func(v any) bool { return v == "en" })
}

// quotedURI returns the address of the quoted post, or "" if this isn't a
// quote. For a quote with images or video attached, the quoted post is one
// level deeper.
func quotedURI(embed map[string]any) string {
	switch str(embed, "$type") {
	case "app.bsky.embed.record":
		return str(obj(embed, "record"), "uri")
	case "app.bsky.embed.recordWithMedia":
		return str(obj(obj(embed, "record"), "record"), "uri")
	}
	return ""
}

// link returns the link a post shares: from a link card (external embed) if
// there is one, otherwise the first link in the text.
func link(record map[string]any) string {
	embed := obj(record, "embed")
	if str(embed, "$type") == "app.bsky.embed.external" {
		if uri := str(obj(embed, "external"), "uri"); uri != "" {
			return uri
		}
	}
	for _, f := range list(record, "facets") {
		facet, _ := f.(map[string]any)
		for _, ft := range list(facet, "features") {
			feature, _ := ft.(map[string]any)
			if str(feature, "$type") == "app.bsky.richtext.facet#link" {
				if uri := str(feature, "uri"); uri != "" {
					return uri
				}
			}
		}
	}
	return ""
}
