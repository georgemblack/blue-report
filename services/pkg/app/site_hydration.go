package app

import (
	"log/slog"

	"github.com/georgemblack/blue-report/pkg/sites"
	"github.com/georgemblack/blue-report/pkg/util"
)

// Given a snapshot of top sites, hydrate it with data from storage. Specifically:
// - Add the title to each top link
func hydrateSites(app App, agg *sites.Aggregation, snapshot sites.Snapshot) (sites.Snapshot, error) {
	for i, site := range snapshot.Sites {
		for j, link := range site.Links {
			link, err := hydrateSiteLink(app, agg, site.Domain, link)
			if err != nil {
				return sites.Snapshot{}, util.WrapErr("failed to hydrate site link", err)
			}
			snapshot.Sites[i].Links[j] = link
		}
	}

	// Find links with missing titles and remove them from the snapshot.
	// These links are likely to be invalid.
	for i, site := range snapshot.Sites {
		updated := make([]sites.Link, 0)

		for _, link := range site.Links {
			if link.Title != "" {
				// Update the link's rank, as removing items from the list may change it
				link.Rank = len(updated) + 1
				updated = append(updated, link)
			}
		}

		snapshot.Sites[i].Links = updated
	}

	return snapshot, nil
}

func hydrateSiteLink(app App, agg *sites.Aggregation, host string, link sites.Link) (sites.Link, error) {
	hashedURL := util.Hash(link.URL)
	stats := agg.Get(host)
	interactions := stats.Get(link.URL).Total()

	// Check whether we have a thumbnail
	thumbnailURL, err := app.Storage.GetThumbnailURL(hashedURL)
	if err != nil {
		slog.Warn(util.WrapErr("failed to check for thumbnail", err).Error(), "url", link.URL)
	}
	link.ThumbnailURL = thumbnailURL

	// Check whether we have a title
	link.Title = getTitle(app.Storage, link.URL)

	// If either title or thumbnail is missing, fetch from CardyB & store
	if link.ThumbnailURL == "" || link.Title == "" {
		metadata := GetCardMetadata(app.Config, link.URL)

		// Save title
		if link.Title == "" && metadata.Title != "" {
			link.Title = formatTitle(metadata.Title)
			updateTitle(app.Storage, link.URL, link.Title)
		}

		// Save thumbnail
		if link.ThumbnailURL == "" && metadata.ImageURL != "" {
			link.ThumbnailURL, err = app.Storage.SaveThumbnail(hashedURL, metadata.ImageURL)
			if err != nil {
				slog.Warn(util.WrapErr("failed to save thumbnail", err).Error(), "url", link.URL)
			}
		}
	}

	link.Interactions = interactions

	slog.Debug("hydrated", "record", link)
	return link, nil
}
