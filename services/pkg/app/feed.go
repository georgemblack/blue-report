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
	feed := feeds.AtomFeed{
		Xmlns: "http://www.w3.org/2005/Atom",
		Title: "The Blue Report",
		Link: &feeds.AtomLink{
			Href: "https://data.theblue.report/feeds/top-day.xml",
			Rel:  "self",
		},
		Id:      "https://data.theblue.report/feeds/top-day.xml",
		Icon:    "https://theblue.report/icons/web-app-manifest-512x512.png",
		Updated: time.Now().UTC().Format(time.RFC3339),
	}

	// Fetch all feed entries
	entries, err := stg.GetFeedEntries()
	if err != nil {
		return "", util.WrapErr("failed to get feed entries", err)
	}

	for _, entry := range entries {
		hostname := urltools.Hostname(entry.Content.URL)
		feed.Entries = append(feed.Entries, &feeds.AtomEntry{
			Id:      entry.Content.URL,
			Title:   entry.Content.Title,
			Links:   []feeds.AtomLink{{Href: entry.Content.URL, Rel: "alternate"}},
			Content: &feeds.AtomContent{Content: generateFeedContent(entry.Content), Type: "html"},
			Author:  &feeds.AtomAuthor{AtomPerson: feeds.AtomPerson{Name: hostname}},
			Updated: entry.Timestamp.Format(time.RFC3339),
		})
	}

	atom, err := feeds.ToXML(&feed)
	if err != nil {
		return "", util.WrapErr("failed to marshal atom feed", err)
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

	for _, post := range content.RecommendedPosts {
		postURL := postURL(post.AtURI, post.Handle)
		result += fmt.Sprintf("<p>Post by <a href=\"https://bsky.app/profile/%s\">@%s</a></p>", post.Handle, post.Handle)
		result += fmt.Sprintf("<blockquote>%s</blockquote>", post.Text)
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
