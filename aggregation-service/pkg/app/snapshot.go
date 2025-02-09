package app

type Snapshot struct {
	GeneratedAt string `json:"generated_at"`
	Links       []Link `json:"links"`
}

type Link struct {
	Rank        int             `json:"rank"`
	URL         string          `json:"url"`
	Title       string          `json:"title"`
	ThumbnailID string          `json:"thumbnail_id"`
	Aggregation LinkAggregation `json:"aggregation"`
}

type LinkAggregation struct {
	Posts   int `json:"posts"`
	Reposts int `json:"reposts"`
	Likes   int `json:"likes"`
	Clicks  int `json:"clicks"`
}
