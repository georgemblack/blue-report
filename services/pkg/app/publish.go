package app

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/georgemblack/blue-report/pkg/links"
	"github.com/georgemblack/blue-report/pkg/sites"
	"github.com/georgemblack/blue-report/pkg/storage"
	"github.com/georgemblack/blue-report/pkg/util"
)

// PublishLinkSnapshot publishes data for the 'top links' report to storage, where it is then read by a static site generator.
// It also updates the feed of top posts, which is used by Atom/JSON generator.
func PublishLinkSnapshot(snapshot links.Snapshot) error {
	slog.Info("publishing snapshot")
	start := time.Now()

	app, err := NewApp()
	if err != nil {
		return util.WrapErr("failed to create app", err)
	}

	// Save data to storage as JSON
	data, err := json.Marshal(snapshot)
	if err != nil {
		return util.WrapErr("failed to marshal snapshot", err)
	}
	err = app.Storage.PublishLinkSnapshot(data)
	if err != nil {
		return util.WrapErr("failed to publish snapshot", err)
	}

	if os.Getenv("DEBUG") == "true" {
		os.WriteFile("dist/snapshot.json", data, 0644)
	}

	// Find the top link over the last 24 hours, and add it to the feed.
	// Skip if there was an entry in the last X hours to avoid spamming the feed.
	topLink := snapshot.TopDayLink()
	if !app.Storage.RecentFeedEntry() && topLink.URL != "" {
		slog.Info("adding feed entry if it doesn't exist", "url", topLink.URL)
		err = app.Storage.AddFeedEntry(storage.FeedEntry{
			Timestamp: time.Now().UTC(),
			Content: storage.FeedEntryContent{
				Title:            topLink.Title,
				URL:              topLink.URL,
				RecommendedPosts: toFeedPosts(topLink.RecommendedPosts),
			},
		})
		if err != nil {
			return util.WrapErr("failed to add feed item", err)
		}

		slog.Info("removing feed entries older than 90 days")
		err = app.Storage.CleanFeed()
		if err != nil {
			slog.Warn("failed to clean feed", "error", err)
		}
	} else {
		slog.Info("skipping feed entry due to cooldown period")
	}

	// Generate Atom and JSON feeds
	atom, err := generateAtomFeed(app.Storage)
	if err != nil {
		return util.WrapErr("failed to generate atom feed", err)
	}

	json, err := generateJSONFeed(app.Storage)
	if err != nil {
		return util.WrapErr("failed to generate json feed", err)
	}

	// Publish Atom and JSON feeds
	err = app.Storage.PublishFeeds(atom, json)
	if err != nil {
		return util.WrapErr("failed to publish feeds", err)
	}

	duration := time.Since(start)
	slog.Info("publish complete", "seconds", duration.Seconds())
	return nil
}

func PublishSiteSnapshot(snapshot sites.Snapshot) error {
	slog.Info("publishing snapshot")
	start := time.Now()

	app, err := NewApp()
	if err != nil {
		return util.WrapErr("failed to create app", err)
	}

	// Save snapshot to storage as JSON
	data, err := json.Marshal(snapshot)
	if err != nil {
		return util.WrapErr("failed to marshal snapshot", err)
	}
	err = app.Storage.PublishSiteSnapshot(data)
	if err != nil {
		return util.WrapErr("failed to publish snapshot", err)
	}

	if os.Getenv("DEBUG") == "true" {
		os.WriteFile("dist/sites.json", data, 0644)
	}

	duration := time.Since(start)
	slog.Info("publish complete", "seconds", duration.Seconds())
	return nil
}

// Deploy the site on CloudFlare Pages by making an HTTP POST request to the deploy webhook.
// The deploy hook URL is considered a secret.
func deploy(hookURL string) error {
	resp, err := http.Post(hookURL, "application/json", nil)
	if err != nil {
		return util.WrapErr("failed to trigger deploy", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return util.WrapErr("failed to trigger deploy", fmt.Errorf("unexpected status code: %d", resp.StatusCode))
	}

	return nil
}

func toFeedPosts(posts []links.Post) []storage.FeedEntryPost {
	var feedPosts []storage.FeedEntryPost
	for _, post := range posts {
		feedPosts = append(feedPosts, storage.FeedEntryPost{
			Rank:     post.Rank,
			AtURI:    post.AtURI,
			Username: post.Username,
			Handle:   post.Handle,
			Text:     post.Text,
		})
	}
	return feedPosts
}
