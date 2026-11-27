// Package publish builds the files the website reads (top-links.json) and
// uploads them.
package publish

import (
	"time"

	"github.com/georgemblack/blue-report/backend/internal/hydrate"
	"github.com/georgemblack/blue-report/backend/internal/ranking"
	"github.com/georgemblack/blue-report/backend/internal/state"
)

// FallbackTitle is shown for links we don't have a title for yet.
const FallbackTitle = "(No Title)"

// Snapshot is top-links.json. Its shape matches v1 exactly, so the website
// only needs its URL changed to switch over.
type Snapshot struct {
	GeneratedAt string `json:"generated_at"`
	TopHour     []Link `json:"top_hour"`
	TopDay      []Link `json:"top_day"`
	TopWeek     []Link `json:"top_week"`
}

type Link struct {
	Rank         int    `json:"rank"`
	URL          string `json:"url"`
	Title        string `json:"title"`
	ThumbnailURL string `json:"thumbnail_url"`

	// Week counts in every list, as in v1.
	PostCount   int `json:"post_count"`
	RepostCount int `json:"repost_count"`
	LikeCount   int `json:"like_count"`

	// Always a list (never null): the website loops over it.
	RecommendedPosts []Post `json:"recommended_posts"`
}

type Post struct {
	Rank     int    `json:"rank"`
	AtURI    string `json:"at_uri"`
	Username string `json:"username"`
	Handle   string `json:"handle"`
	Text     string `json:"text"`
}

// BuildSnapshot turns the actor's rankings into top-links.json, adding each
// link's title, thumbnail and recommended posts from info (keyed by URL).
// Links missing from info get the fallback title and nothing else.
func BuildSnapshot(res state.TopResult, info map[string]hydrate.Info, generatedAt time.Time) Snapshot {
	return Snapshot{
		GeneratedAt: generatedAt.UTC().Format(time.RFC3339),
		TopHour:     links(res.Ranking.Hour, info),
		TopDay:      links(res.Ranking.Day, info),
		TopWeek:     links(res.Ranking.Week, info),
	}
}

func links(entries []ranking.Entry, info map[string]hydrate.Info) []Link {
	out := make([]Link, 0, len(entries))
	for i, e := range entries {
		meta := info[e.URL]
		link := Link{
			Rank:             i + 1,
			URL:              e.URL,
			Title:            meta.Title,
			ThumbnailURL:     meta.ThumbnailURL,
			PostCount:        e.Week.Posts,
			RepostCount:      e.Week.Reposts,
			LikeCount:        e.Week.Likes,
			RecommendedPosts: []Post{},
		}
		if link.Title == "" {
			link.Title = FallbackTitle
		}
		for j, p := range meta.Posts {
			link.RecommendedPosts = append(link.RecommendedPosts, Post{
				Rank: j + 1, AtURI: p.AtURI, Username: p.Username, Handle: p.Handle, Text: p.Text,
			})
		}
		out = append(out, link)
	}
	return out
}

// allEntries returns the links in every list, for hydrating.
func allEntries(r ranking.Result) []ranking.Entry {
	return append(append(append([]ranking.Entry{}, r.Hour...), r.Day...), r.Week...)
}
