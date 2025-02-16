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
			// Fetch the title from storage
			metadata, err := stg.GetURLMetadata(link.URL)
			if err != nil {
				var notFoundEx *types.ResourceNotFoundException
				if !errors.As(err, &notFoundEx) {
					slog.Warn(util.WrapErr("failed to get url metadata", err).Error(), "url", link.URL)
				}
			}

			if metadata.Title == "" {
				snapshot.Sites[i].Links[j].Title = link.URL
			} else {
				snapshot.Sites[i].Links[j].Title = metadata.Title
			}
		}
	}

	return snapshot, nil
}
