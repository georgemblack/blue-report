// Package event holds the types shared by everything that handles Bluesky events.
package event

import "time"

// Type is the kind of interaction. The numbers match the stored event schema
// (docs/event-schema.md), so they must never change.
type Type uint8

const (
	Post   Type = 0 // includes quote posts
	Repost Type = 1
	Like   Type = 2
)

// Event is one stored interaction: a row in the events files described in
// docs/event-schema.md.
type Event struct {
	TimeUS     int64  // when it happened on Bluesky, in microseconds since the Unix epoch
	Type       Type   // quotes are stored as posts
	DID        string // the user who made the post, repost or like
	RKey       string // the record key of that user's own post, repost or like
	SubjectURI string // the post that shared the link
	URL        string // the link, cleaned but not followed through redirects
}

// Seconds converts a time to whole seconds since the Unix epoch, as a uint32.
// This keeps per-entry memory small, and lasts until the year 2106.
func Seconds(t time.Time) uint32 {
	s := t.Unix()
	if s < 0 {
		return 0
	}
	return uint32(s)
}
