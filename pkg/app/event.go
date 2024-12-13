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
