package app

import "github.com/georgemblack/blue-report/pkg/app/util"

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
