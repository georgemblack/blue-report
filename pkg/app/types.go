package app

// Count represents an a set of aggregated counts for a given URL.
// i.e. how many times it was posted, reposted, as well as its score.
type Count struct {
	PostCount   int
	RepostCount int
}

// Score determins a URL's rank on the final report.
// At the moment, a post is worth 10 'points', and a repost is worth 1.
func (c Count) Score() int {
	return c.PostCount*10 + c.RepostCount
}

// Report represents all data requried to render the webpage.
type Report struct {
	Links       []ReportItems
	GeneratedAt string
}

// ReportItems represents a single item to be rendered on the webpage
type ReportItems struct {
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
