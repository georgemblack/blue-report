package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/georgemblack/blue-report/pkg/storage"
	"github.com/georgemblack/blue-report/pkg/urltools"
	"github.com/georgemblack/blue-report/pkg/util"
	"github.com/gorilla/feeds"
)

func generateAtomFeed(stg Storage) (string, error) {
	feed := feeds.Feed{
		Id:    "https://theblue.report/",
		Title: "The Blue Report",
		Link: &feeds.Link{
			Href: "https://data.theblue.report/feeds/top-day.xml",
			Rel:  "self",
		},
		Updated: time.Now().UTC(),
	}

	// Fetch all feed entries
	entries, err := stg.GetFeedEntries()
	if err != nil {
		return "", util.WrapErr("failed to get feed entries", err)
	}

	for _, entry := range entries {
		hostname := urltools.Hostname(entry.Content.URL)

		feed.Add(&feeds.Item{
			Id:      entry.Content.URL,
			Title:   entry.Content.Title,
			Link:    &feeds.Link{Href: entry.Content.URL},
			Content: generateFeedContent(entry.Content),
			Author:  &feeds.Author{Name: hostname},
			Updated: entry.Timestamp,
		})
	}

	atom, err := feed.ToAtom()
	if err != nil {
		return "", util.WrapErr("failed to generate atom feed", err)
	}

	return atom, nil
}

type JSONFeed struct {
	Version     string         `json:"version"`
	Title       string         `json:"title"`
	HomePageURL string         `json:"home_page_url"`
	FeedURL     string         `json:"feed_url"`
	Description string         `json:"description"`
	Icon        string         `json:"icon"`
	Language    string         `json:"language"`
	Items       []JSONFeedItem `json:"items"`
}

type JSONFeedItem struct {
	ID            string `json:"id"`
	URL           string `json:"url"`
	Title         string `json:"title"`
	ContentHTML   string `json:"content_html"`
	ContentText   string `json:"content_text"`
	DatePublished string `json:"date_published"`
}

func generateJSONFeed(stg Storage) (string, error) {
	feed := JSONFeed{
		Version:     "https://jsonfeed.org/version/1.1",
		Title:       "The Blue Report",
		HomePageURL: "https://theblue.report",
		FeedURL:     "https://data.theblue.report/feeds/top-day.json",
		Description: "The top links on Bluesky over the past day",
		Icon:        "https://theblue.report/icons/web-app-manifest-512x512.png",
		Language:    "en",
	}

	// Fetch all feed entries
	entries, err := stg.GetFeedEntries()
	if err != nil {
		return "", util.WrapErr("failed to get feed entries", err)
	}

	for _, entry := range entries {
		feed.Items = append(feed.Items, JSONFeedItem{
			ID:            entry.Content.URL,
			URL:           entry.Content.URL,
			Title:         entry.Content.Title,
			ContentHTML:   generateFeedContent(entry.Content),
			DatePublished: entry.Timestamp.Format(time.RFC3339),
		})
	}

	data, err := json.Marshal(feed)
	if err != nil {
		return "", util.WrapErr("failed to marshal json feed", err)
	}

	return string(data), nil
}

func generateFeedContent(content storage.FeedEntryContent) string {
	result := fmt.Sprintf("<p>Trending on Bluesky: <a href=\"%s\">%s</a></p>", content.URL, content.Title)
	if len(content.RecommendedPosts) == 0 {
		return result
	}

	result += "<p>Recommended posts</p>"
	for _, post := range content.RecommendedPosts {
		postURL := postURL(post.AtURI, post.Handle)
		result += fmt.Sprintf("<blockquote>%s<cite><a href=\"https://bsky.app/profile/%s\">%s</a></cite></blockquote>", post.Text, post.Handle, post.Handle)
		result += fmt.Sprintf("<p><a href=\"%s\">View Post</a></p>", postURL)
	}

	return result
}

// Convert an AT URI & user handle to a URL
func postURL(atURI, handle string) string {
	// Parse rkey from AT URI
	// 'at://did:plc:y5xyloyy7s4a2bwfeimj7r3b/app.bsky.feed.post/3lhrms2lbc22c' -> '3lhrms2lbc22c'
	parts := strings.Split(atURI, "/")
	rkey := parts[len(parts)-1]

	// Construct URL
	return fmt.Sprintf("https://bsky.app/profile/%s/post/%s", handle, rkey)
}
