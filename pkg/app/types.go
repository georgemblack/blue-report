package app

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
	URI string `json:"uri"`
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

// InternalRecord represents a record stored in Valkey.
type InternalRecord struct {
	Type int // 0: post, 1: repost
	URL  string
	DID  string
}

func (r InternalRecord) Valid() bool {
	if r.Type != 0 && r.Type != 1 {
		return false
	}
	if r.URL == "" {
		return false
	}
	if r.DID == "" {
		return false
	}
	return true
}
