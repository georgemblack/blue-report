package app

import (
	"errors"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/service/sso/types"
	"github.com/georgemblack/blue-report/pkg/sites"
	"github.com/georgemblack/blue-report/pkg/util"
)

// Given a snapshot of top sites, hydrate it with data from storage. Specifically:
// - Add the title to each top link
func hydrateSites(stg Storage, snapshot sites.Snapshot) (sites.Snapshot, error) {
	for i, site := range snapshot.Sites {
		for j, link := range site.Links {
			title := ""

			// Fetch the title from storage
			metadata, err := stg.GetURLMetadata(link.URL)
			if err != nil {
				var notFoundEx *types.ResourceNotFoundException
				if !errors.As(err, &notFoundEx) {
					slog.Warn(util.WrapErr("failed to get url metadata", err).Error(), "url", link.URL)
				}
			}
			title = metadata.Title

			// If the title is empty, attempt to fetch it from CardyB
			if title == "" {
				cardy, err := cardyB(link.URL)
				if err != nil {
					slog.Warn(util.WrapErr("failed to get title from cardyb", err).Error(), "url", link.URL)
				}

				// If we found a title, update storage
				if cardy.Title != "" {
					title = formatTitle(cardy.Title)
					updateTitle(stg, link.URL, title)
				}
			}

			snapshot.Sites[i].Links[j].Title = title
		}
	}

	return snapshot, nil
}
