package app

import (
	"errors"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/service/sso/types"
	"github.com/georgemblack/blue-report/pkg/storage"
	"github.com/georgemblack/blue-report/pkg/util"
)

func hydrate(app App, agg Aggregation, snapshot LinkSnapshot) (LinkSnapshot, error) {
	for i := range snapshot.Links {
		link, err := hydrateLink(app, agg, i, snapshot.Links[i])
		if err != nil {
			return LinkSnapshot{}, util.WrapErr("failed to hydrate link", err)
		}
		snapshot.Links[i] = link
	}

	return snapshot, nil
}

func hydrateLink(app App, agg Aggregation, index int, link Link) (Link, error) {
	hashedURL := util.Hash(link.URL)
	record, err := app.Cache.ReadURL(hashedURL)
	if err != nil {
		return Link{}, util.WrapErr("failed to read url record", err)
	}

	// Fetch the thumbnail from the Bluesky CDN and store it in our S3 bucket.
	// The thumbnail ID is the hash of the URL.
	if record.ImageURL != "" {
		err := app.Storage.SaveThumbnail(hashedURL, record.ImageURL)
		if err != nil {
			slog.Warn(util.WrapErr("failed to save thumbnail", err).Error(), "url", link.URL)
		}
	}

	// Set the thumbnail ID if it exists
	exists, err := app.Storage.ThumbnailExists(hashedURL)
	if err != nil {
		slog.Warn(util.WrapErr("failed to check for thumbnail", err).Error(), "url", link.URL)
	} else if exists {
		link.ThumbnailID = hashedURL
	}

	// Fetch the title from storage.
	// If we don't have a title, use the title in the cache .
	link.Title = getTitle(app.Storage, link.URL)
	if link.Title == "" {
		link.Title = record.Title
	}

	// Update storage with the latest title.
	// Even if it's empty, having the record in storage allows us to manually add a title if it is missing.
	updateTitle(app.Storage, link.URL, link.Title)

	// Set display items, such as rank, title, host, and stats
	if link.Title == "" {
		link.Title = "(No Title)"
	}
	link.Rank = index + 1
	link.PostCount = record.Totals.Posts
	link.RepostCount = record.Totals.Reposts
	link.LikeCount = record.Totals.Likes
	link.ClickCount = clicks(link.URL)

	// Generate a list of the most popular 2-5 posts referencing the URL.
	// Posts should contain commentary on the subject of the link.
	aggregationItem := agg[link.URL]
	link.RecommendedPosts = recommendedPosts(app.Bluesky, aggregationItem.TopPosts())

	slog.Debug("hydrated", "record", link)
	return link, nil
}

func getTitle(stg Storage, url string) string {
	metadata, err := stg.GetURLMetadata(url)
	if err != nil {
		var notFoundEx *types.ResourceNotFoundException
		if errors.As(err, &notFoundEx) {
			return ""
		} else {
			slog.Warn(util.WrapErr("failed to get url metadata", err).Error(), "url", url)
		}
	}

	return metadata.Title
}

func updateTitle(stg Storage, url string, title string) {
	err := stg.SaveURLMetadata(storage.URLMetadata{
		URL:   url,
		Title: title,
	})
	if err != nil {
		slog.Warn(util.WrapErr("failed to save url metadata", err).Error(), "url", url)
	}
}
