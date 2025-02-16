package sites

import "time"

func NewSnapshot() Snapshot {
	return Snapshot{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

type Snapshot struct {
	GeneratedAt string `json:"generated_at"`
	Sites       []Site `json:"sites"`
}

type Site struct {
	Rank         int    `json:"rank"`
	Name         string `json:"name"`
	Domain       string `json:"domain"`
	Interactions int    `json:"interactions"`
	Links        []Link `json:"links"`
}

type Post struct {
	AtURI    string `json:"at_uri"`
	Username string `json:"username"`
	Handle   string `json:"handle"`
	Text     string `json:"text"`
}

type Link struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

func (s *Snapshot) AddSite(domain string, agg AggregationItem) {
	urls := agg.TopLinks(6)

	links := make([]Link, 0, len(urls))
	for _, url := range urls {
		links = append(links, Link{
			URL: url,
		})
	}

	site := Site{
		Rank:         len(s.Sites) + 1,
		Name:         domain,
		Domain:       domain,
		Interactions: agg.counts.Total(),
		Links:        links,
	}

	s.Sites = append(s.Sites, site)
}
