package app

import "time"

// Create an empty snapshot populated with the 'GeneratedAt' timestamp, as well as a list of links w/URLs
func newLinkSnapshot(urls []string) LinkSnapshot {
	links := make([]Link, 0, len(urls))

	for _, url := range urls {
		links = append(links, Link{
			URL: url,
		})
	}

	return LinkSnapshot{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Links:       links,
	}
}

type LinkSnapshot struct {
	GeneratedAt string `json:"generated_at"`
	Links       []Link `json:"links"`
}

type Link struct {
	Rank             int    `json:"rank"`
	URL              string `json:"url"`
	Title            string `json:"title"`
	ThumbnailID      string `json:"thumbnail_id"`
	PostCount        int    `json:"post_count"`
	RepostCount      int    `json:"repost_count"`
	LikeCount        int    `json:"like_count"`
	ClickCount       int    `json:"click_count"`
	RecommendedPosts []Post `json:"recommended_posts"`
}

type Post struct {
	AtURI    string `json:"at_uri"`
	Username string `json:"username"`
	Handle   string `json:"handle"`
	Text     string `json:"text"`
}
