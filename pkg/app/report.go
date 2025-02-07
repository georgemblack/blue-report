package app

// Aggregation represents an a set of aggregated counts for a given URL.
// i.e. how many times it was posted, reposted, as well as its score.
type Aggregation struct {
	Posts   int
	Reposts int
	Likes   int
}

// Score determins a URL's rank on the final report.
// A post is worth 10 points, a repost 10 points, and a like 1 point.
func (c Aggregation) Score() int {
	return (c.Posts * 10) + (c.Reposts * 10) + c.Likes
}

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
}

// ReportItem represents a single item to be rendered on the webpage
type ReportItem struct {
	Rank         int
	URL          string
	EscapedURL   string
	Host         string
	Title        string
	ThumbnailURL string
	Aggregation  Aggregation
	Display      Display
}
