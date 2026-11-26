// Package jetstream turns raw Bluesky Jetstream v2 messages into the few
// kinds of interaction we care about: posts, quote posts, reposts and likes.
//
// Format reference: https://bsky.network/docs/jetstream/
package jetstream

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"
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
	Seq    int64  // Jetstream's sequence number for the event
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

// Info is a notice from Jetstream rather than an event. For example, name
// "OutdatedCursor" means the requested cursor was older than Jetstream keeps,
// so some events were missed. Parse returns it as an error so it gets logged.
type Info struct {
	Name    string
	Message string
}

func (i *Info) Error() string {
	return fmt.Sprintf("jetstream info %s: %s", i.Name, i.Message)
}

// Parse returns the interaction in a Jetstream message. ok is false when the
// message isn't one we count: anything other than a newly created post,
// repost or like, or a post not marked as English. An error means the
// message couldn't be read, or was an Info notice.
func Parse(data []byte) (msg Message, ok bool, err error) {
	var frame rawFrame
	if err := json.Unmarshal(data, &frame); err != nil {
		return Message{}, false, fmt.Errorf("parsing jetstream message: %w", err)
	}
	if frame.Type != "message" {
		return Message{}, false, fmt.Errorf("unexpected jetstream frame type %q", frame.Type)
	}

	ev := frame.Payload
	switch ev.Type {
	case "network.bsky.jetstream.subscribeEvents#commit":
	case "network.bsky.jetstream.subscribeEvents#info":
		return Message{}, false, &Info{Name: ev.Name, Message: ev.Message}
	default:
		return Message{}, false, nil // identity, account and sync events
	}

	if ev.Operation != "create" || ev.DID == "" || ev.RKey == "" {
		return Message{}, false, nil
	}

	// Check the collection before decoding the record, since records from
	// other collections have different shapes.
	var kind Kind
	switch ev.Collection {
	case "app.bsky.feed.post":
		kind = KindPost
	case "app.bsky.feed.repost":
		kind = KindRepost
	case "app.bsky.feed.like":
		kind = KindLike
	default:
		return Message{}, false, nil
	}

	t, err := time.Parse(time.RFC3339Nano, ev.Time)
	if err != nil {
		return Message{}, false, fmt.Errorf("parsing jetstream event time: %w", err)
	}

	var record rawRecord
	if err := json.Unmarshal(ev.Record, &record); err != nil {
		return Message{}, false, fmt.Errorf("parsing %s record: %w", ev.Collection, err)
	}

	msg = Message{Seq: ev.Seq, TimeUS: t.UnixMicro(), DID: ev.DID, RKey: ev.RKey, Kind: kind}

	if kind != KindPost {
		if record.Subject.URI == "" {
			return Message{}, false, nil
		}
		msg.SubjectURI = record.Subject.URI
		return msg, true, nil
	}

	if !slices.Contains(record.Langs, "en") {
		return Message{}, false, nil
	}
	if quoted := record.Embed.quotedURI(); quoted != "" {
		msg.Kind = KindQuote
		msg.SubjectURI = quoted
		return msg, true, nil
	}
	msg.SubjectURI = fmt.Sprintf("at://%s/app.bsky.feed.post/%s", ev.DID, ev.RKey)
	msg.URL = record.link()
	return msg, true, nil
}

// The parts of a Jetstream message we read. Everything else is ignored.
// Each message is a frame with the event under "payload".
type rawFrame struct {
	Type    string     `json:"$type"`
	Payload rawPayload `json:"payload"`
}

type rawPayload struct {
	Type string `json:"$type"`

	// Commit events
	DID        string          `json:"did"`
	Seq        int64           `json:"seq"`
	Time       string          `json:"time"` // RFC 3339, when Jetstream saw the event
	Operation  string          `json:"operation"`
	Collection string          `json:"collection"`
	RKey       string          `json:"rkey"`
	Record     json.RawMessage `json:"record"`

	// Info notices
	Name    string `json:"name"`
	Message string `json:"message"`
}

type rawRecord struct {
	Langs  []string `json:"langs"`
	Embed  rawEmbed `json:"embed"`
	Facets []struct {
		Features []struct {
			Type string `json:"$type"`
			URI  string `json:"uri"`
		} `json:"features"`
	} `json:"facets"`
	Subject struct {
		URI string `json:"uri"`
	} `json:"subject"`
}

type rawEmbed struct {
	Type     string `json:"$type"`
	External struct {
		URI string `json:"uri"`
	} `json:"external"`
	// For a quote, "record" holds the quoted post. For a quote with images
	// or video attached, the quoted post is one level deeper.
	Record struct {
		URI    string `json:"uri"`
		Record struct {
			URI string `json:"uri"`
		} `json:"record"`
	} `json:"record"`
}

// quotedURI returns the address of the quoted post, or "" if this isn't a quote.
func (e rawEmbed) quotedURI() string {
	switch e.Type {
	case "app.bsky.embed.record":
		return e.Record.URI
	case "app.bsky.embed.recordWithMedia":
		return e.Record.Record.URI
	}
	return ""
}

// link returns the link a post shares: from a link card (external embed) if
// there is one, otherwise the first link in the text.
func (r rawRecord) link() string {
	if r.Embed.Type == "app.bsky.embed.external" && r.Embed.External.URI != "" {
		return r.Embed.External.URI
	}
	for _, facet := range r.Facets {
		for _, feature := range facet.Features {
			if feature.Type == "app.bsky.richtext.facet#link" && feature.URI != "" {
				return feature.URI
			}
		}
	}
	return ""
}
