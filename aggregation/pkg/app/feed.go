package app

import (
	"encoding/json"
	"fmt"
	"time"

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
		title := getTitle(stg, entry.URL)
		hostname := urltools.Hostname(entry.URL)

		feed.Add(&feeds.Item{
			Id:      entry.URL,
			Title:   getTitle(stg, entry.URL),
			Link:    &feeds.Link{Href: entry.URL},
			Content: generateFeedContent(entry.URL, title),
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
		title := getTitle(stg, entry.URL)

		feed.Items = append(feed.Items, JSONFeedItem{
			ID:            entry.URL,
			URL:           entry.URL,
			Title:         title,
			ContentHTML:   generateFeedContent(entry.URL, title),
			DatePublished: entry.Timestamp.Format(time.RFC3339),
		})
	}

	data, err := json.Marshal(feed)
	if err != nil {
		return "", util.WrapErr("failed to marshal json feed", err)
	}

	return string(data), nil
}

func generateFeedContent(url, title string) string {
	return fmt.Sprintf("<p>Trending on Bluesky: <a href=\"%s\">%s</a></p>", url, title)
}
