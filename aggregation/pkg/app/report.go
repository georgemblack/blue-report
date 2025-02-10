package app

// Display represents numbers that are formatted for rendering in the template.
// ie. "1,000,000"
type Display struct {
	Posts   string
	Reposts string
	Likes   string
	Clicks  string
}

// Report represents all data requried to render the webpage.
type Report struct {
	Items       []ReportItem
	GeneratedAt string
	Archive     bool // Whether or not to generate the page for the archive
}

// ReportItem represents a single item to be rendered on the webpage
type ReportItem struct {
	Rank         int
	URL          string
	EscapedURL   string
	Host         string
	Title        string
	ThumbnailURL string
	Aggregation  URLAggregation
	Display      Display
}
