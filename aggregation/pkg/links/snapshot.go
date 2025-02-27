package links

import "time"

func NewSnapshot() Snapshot {
	return Snapshot{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

type Snapshot struct {
	GeneratedAt string `json:"generated_at"`
	Links       []Link `json:"links"` // Kept for backwards compatibility: https://bsky.app/profile/brianell.in/post/3lhw7kyaqgc2l
	TopHour     []Link `json:"top_hour"`
	TopDay      []Link `json:"top_day"`
	TopWeek     []Link `json:"top_week"`
}

type Link struct {
	Rank             int    `json:"rank"`
	URL              string `json:"url"`
	Title            string `json:"title"`
	ThumbnailID      string `json:"thumbnail_id"`
	PostCount        int    `json:"post_count"`
	RepostCount      int    `json:"repost_count"`
	LikeCount        int    `json:"like_count"`
	RecommendedPosts []Post `json:"recommended_posts"`
}

type Post struct {
	AtURI    string `json:"at_uri"`
	Username string `json:"username"`
	Handle   string `json:"handle"`
	Text     string `json:"text"`
}
