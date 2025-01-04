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
	Type     string   `json:"$type"`
	External External `json:"external"`
}

type External struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	URI         string `json:"uri"`
	Thumb       Thumb  `json:"thumb"`
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

func (s *StreamEvent) isPost() bool {
	return s.Commit.Record.Type == "app.bsky.feed.post"
}

func (s *StreamEvent) isRepost() bool {
	return s.Commit.Record.Type == "app.bsky.feed.repost"
}

func (s *StreamEvent) isLike() bool {
	return s.Commit.Record.Type == "app.bsky.feed.like"
}

func (s *StreamEvent) typeOf() int {
	if s.isPost() {
		return 0
	}
	if s.isRepost() {
		return 1
	}
	if s.isLike() {
		return 2
	}
	return -1
}

func (s *StreamEvent) isEnglish() bool {
	return util.Contains(s.Commit.Record.Languages, "en")
}

func (s *StreamEvent) valid() bool {
	if s.Kind != "commit" {
		return false
	}
	if s.Commit.Operation != "create" {
		return false
	}
	if !s.isPost() && !s.isRepost() && !s.isLike() {
		return false
	}
	if s.isPost() && !s.isEnglish() {
		return false
	}
	return true
}

// Parse a post even to extract the URL, title, and image.
// Post events without embeds will only return a URL.
func (s *StreamEvent) parsePost() (string, string, string) {
	if !s.isPost() {
		return "", "", ""
	}

	// Search embed for URL, title, and image
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

	// Otherwise, search for link facet
	for _, facet := range s.Commit.Record.Facets {
		for _, feature := range facet.Features {
			if feature.Type == "app.bsky.richtext.facet#link" && feature.URI != "" {
				return feature.URI, "", ""
			}
		}
	}

	return "", "", ""
}
