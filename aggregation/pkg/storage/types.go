package storage

import "time"

type EventRecord struct {
	Type      int       `json:"type"` // 0 = post, 1 = repost, 2 = like
	URL       string    `json:"url"`
	DID       string    `json:"did"`
	Timestamp time.Time `json:"timestamp"`
	Post      string    `json:"post"` // AT URI of the post that was created/liked/reposted
}

func (s EventRecord) IsPost() bool {
	return s.Type == 0
}

func (s EventRecord) IsRepost() bool {
	return s.Type == 1
}

func (s EventRecord) IsLike() bool {
	return s.Type == 2
}

type URLMetadata struct {
	URL   string
	Title string
}
