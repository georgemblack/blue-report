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

// Seconds converts a time to whole seconds since the Unix epoch, as a uint32.
// This keeps per-entry memory small, and lasts until the year 2106.
func Seconds(t time.Time) uint32 {
	s := t.Unix()
	if s < 0 {
		return 0
	}
	return uint32(s)
}
