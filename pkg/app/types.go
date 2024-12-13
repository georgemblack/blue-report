package app

import "time"

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

func (s *StreamEvent) isEnglish() bool {
	return contains(s.Commit.Record.Languages, "en")
}

// Aggregration represents a set of URLs and metadata for each.
type Aggregation struct {
	Items []AggregationItem
}

type AggregationItem struct {
	URL   string
	Count int
}

type URLRecord struct {
	URL         string `msgpack:"u"`
	Title       string `msgpack:"t"`
	Description string `msgpack:"d"`
	ImageURL    string `msgpack:"p"`
}

func (r URLRecord) MissingURL() bool {
	return r.URL == ""
}

func (r URLRecord) MissingFields() bool {
	if (r.URL == "") || (r.Title == "") || (r.Description == "") || (r.ImageURL == "") {
		return true
	}
	return false
}

func (r URLRecord) Complete() bool {
	return !r.MissingFields()
}

// EventRecord represents a record stored in Valkey.
// This struct is used to serialize and deserialize records.
type EventRecord struct {
	Type    int    `msgpack:"t"` // 0: post, 1: repost
	URLHash string `msgpack:"u"`
	DID     string `msgpack:"d"`
}

func (r EventRecord) isPost() bool {
	return r.Type == 0
}

func (r EventRecord) isRepost() bool {
	return r.Type == 1
}

func (r EventRecord) Valid() bool {
	if r.Type != 0 && r.Type != 1 {
		return false
	}
	if r.URLHash == "" {
		return false
	}
	if r.DID == "" {
		return false
	}
	return true
}

func (r EventRecord) Empty() bool {
	return !r.Valid()
}

// InternalCacheRecord represents a record stored in our internal, in-memory cache.
type InternalCacheRecord struct {
	Record EventRecord
	Expiry time.Time
}

func (r InternalCacheRecord) Expired() bool {
	return time.Now().After(r.Expiry)
}

type Count struct {
	PostCount   int
	RepostCount int
	Score       int
}

// Report represents all data requried to render the webpage.
type Report struct {
	Links       []ReportLinks
	GeneratedAt string
}

// ReportLinks represents a single item to be rendered on the webpage
type ReportLinks struct {
	Rank           int
	URLHash        string
	URL            string
	Host           string
	Title          string
	Description    string
	ImageURL       string
	Count          Count
	PostCountStr   string
	RepostCountStr string
}

type Dump struct {
	Items []DumpItem `json:"items"`
}

type DumpItem struct {
	Key   string              `json:"key"`
	Value InternalCacheRecord `json:"value"`
}
