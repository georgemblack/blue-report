package app

import (
	"fmt"

	"github.com/georgemblack/blue-report/pkg/app/util"
)

// StreamEvent (and subtypes) represent a message from the Jetstream.
// Fields for both posts and reposts are included.
type StreamEvent struct {
	DID    string `json:"did"`
	Kind   string `json:"kind"`
	Commit Commit `json:"commit"`
}

type Commit struct {
	Operation string `json:"operation"`
	Record    Record `json:"record"`
	CID       string `json:"cid"`
}

type Record struct {
	Type      string   `json:"$type"`
	Languages []string `json:"langs"`
	Embed     Embed    `json:"embed"`
	Facets    []Facet  `json:"facets"`
	Subject   Subject  `json:"subject"`
}

type Embed struct {
	Type     string        `json:"$type"`
	External ExternalEmbed `json:"external"`
	Record   RecordEmbed   `json:"record"`
}

type ExternalEmbed struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	URI         string `json:"uri"`
	Thumb       Thumb  `json:"thumb"`
}

type RecordEmbed struct {
	CID string `json:"cid"`
}

type Thumb struct {
	Type     string `json:"$type"`
	Ref      Ref    `json:"ref"`
	MimeType string `json:"mimeType"`
}

type Ref struct {
	Link string `json:"$link"`
}

type Facet struct {
	Features []Feature `json:"features"`
}

type Subject struct {
	CID string `json:"cid"`
}

type Feature struct {
	Type string `json:"$type"`
	URI  string `json:"uri"`
}

// Valid determines whether a stream event can be processed by our application.
func (s *StreamEvent) Valid() bool {
	if s.Kind != "commit" {
		return false
	}
	if s.Commit.Operation != "create" {
		return false
	}
	if !s.IsPost() && !s.IsRepost() && !s.IsLike() {
		return false
	}
	if s.IsPost() && !s.IsEnglish() {
		return false
	}
	return true
}

func (s *StreamEvent) IsPost() bool {
	return s.Commit.Record.Type == "app.bsky.feed.post"
}

// IsQuotePost determines whether the event is a quote post.
// Quote posts are a subset of posts that contain record embeds.
func (s *StreamEvent) IsQuotePost() bool {
	if !s.IsPost() {
		return false
	}
	return s.Commit.Record.Embed.Type == "app.bsky.embed.record"
}

func (s *StreamEvent) IsRepost() bool {
	return s.Commit.Record.Type == "app.bsky.feed.repost"
}

func (s *StreamEvent) IsLike() bool {
	return s.Commit.Record.Type == "app.bsky.feed.like"
}

func (s *StreamEvent) IsEnglish() bool {
	return util.Contains(s.Commit.Record.Languages, "en")
}

// Parse a post to extract the URL, title, and image.
// The URL may be in an embed, or a link facet.
func (s *StreamEvent) ParsePost() (string, string, string) {
	if !s.IsPost() {
		return "", "", ""
	}

	// Search for an external embed
	embed := s.Commit.Record.Embed
	if embed.Type == "app.bsky.embed.external" {
		uri := embed.External.URI
		title := embed.External.Title
		image := ""

		// Add image if it exists
		thumb := embed.External.Thumb
		if thumb.Type == "blob" && thumb.MimeType == "image/jpeg" {
			image = fmt.Sprintf("https://cdn.bsky.app/img/feed_thumbnail/plain/%s/%s", s.DID, thumb.Ref.Link)
		}

		return uri, title, image
	}

	// Search for a link facet
	for _, facet := range s.Commit.Record.Facets {
		for _, feature := range facet.Features {
			if feature.Type == "app.bsky.richtext.facet#link" && feature.URI != "" {
				return feature.URI, "", ""
			}
		}
	}

	return "", "", ""
}

func (s *StreamEvent) TypeOf() int {
	if s.IsPost() {
		return 0
	}
	if s.IsRepost() {
		return 1
	}
	if s.IsLike() {
		return 2
	}
	return -1
}
