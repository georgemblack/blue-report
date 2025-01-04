package app

// Count represents an a set of aggregated counts for a given URL.
// i.e. how many times it was posted, reposted, as well as its score.
type Count struct {
	PostCount   int
	RepostCount int
	LikeCount   int
}

// Score determins a URL's rank on the final report.
// A post is worth 10 points, a repost 10 points, and a like 1 point.
func (c Count) Score() int {
	return (c.PostCount * 10) + (c.RepostCount * 10) + c.LikeCount
}

// Report represents all data requried to render the webpage.
type Report struct {
	NewsItems       []ReportItem // News articles
	EverythingItems []ReportItem // Everything but news articles
	GeneratedAt     string
}

// ReportItem represents a single item to be rendered on the webpage
type ReportItem struct {
	Rank           int
	URL            string
	Host           string
	Title          string
	ThumbnailURL   string
	Count          Count
	PostCountStr   string
	RepostCountStr string
	LikeCountStr   string
}
