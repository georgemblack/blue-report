package app

import (
	"log/slog"

	"github.com/georgemblack/blue-report/pkg/sites"
	"github.com/georgemblack/blue-report/pkg/util"
)

// Given a snapshot of top sites, hydrate it with data from storage. Specifically:
// - Add the title to each top link
func hydrateSites(stg Storage, agg *sites.Aggregation, snapshot sites.Snapshot) (sites.Snapshot, error) {
	for i, site := range snapshot.Sites {
		for j, link := range site.Links {
			link, err := hydrateSiteLink(stg, agg, site.Domain, link)
			if err != nil {
				return sites.Snapshot{}, util.WrapErr("failed to hydrate site link", err)
			}
			snapshot.Sites[i].Links[j] = link
		}
	}

	return snapshot, nil
}

func hydrateSiteLink(stg Storage, agg *sites.Aggregation, host string, link sites.Link) (sites.Link, error) {
	hashedURL := util.Hash(link.URL)
	stats := agg.Get(host)
	interactions := stats.Get(link.URL).Total()

	// Check whether we have a thumbnail
	thumbnailExists, err := stg.ThumbnailExists(hashedURL)
	if err != nil {
		slog.Warn(util.WrapErr("failed to check for thumbnail", err).Error(), "url", link.URL)
	}
	if thumbnailExists {
		link.ThumbnailID = hashedURL
	}

	// Check whether we have a title
	titleExists := false
	if link.Title = getTitle(stg, link.URL); link.Title != "" {
		titleExists = true
	}

	// If either title or thumbnail is missing, fetch from CardyB & store
	if !thumbnailExists || !titleExists {
		cardy, err := cardyB(link.URL)
		if err != nil {
			slog.Warn(util.WrapErr("failed to get title from cardyb", err).Error(), "url", link.URL)
		}

		// Save title
		if !titleExists && cardy.Title != "" {
			link.Title = formatTitle(cardy.Title)
			updateTitle(stg, link.URL, link.Title)
		}

		// Save thumbnail
		if !thumbnailExists && cardy.Image != "" {
			err := stg.SaveThumbnail(hashedURL, cardy.Image)
			if err != nil {
				slog.Warn(util.WrapErr("failed to save thumbnail", err).Error(), "url", link.URL)
			} else {
				link.ThumbnailID = hashedURL
			}
		}
	}

	if link.Title == "" {
		link.Title = "(No Title)"
	}
	link.Interactions = interactions

	slog.Debug("hydrated", "record", link)
	return link, nil
}
