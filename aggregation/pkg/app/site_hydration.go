package app

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

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
				title, err = cardyB(link.URL)
				if err != nil {
					slog.Warn(util.WrapErr("failed to get title from cardyb", err).Error(), "url", link.URL)
				}

				// If we found a title, update storage
				if title != "" {
					updateTitle(stg, link.URL, title)
				}
			}

			snapshot.Sites[i].Links[j].Title = title
		}
	}

	return snapshot, nil
}

type CardyB struct {
	Title string `json:"title"`
	Image string `json:"image"`
}

func cardyB(url string) (string, error) {
	// Fetch the title from CardyB
	// If the title is empty, return the URL
	resp, err := http.Get("https://cardyb.bsky.app/v1/extract?url=" + url)
	if err != nil {
		return "", util.WrapErr("failed to get title from cardyb", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", errors.New("failed to get title from cardyb: status code " + resp.Status)
	}

	var cardyB CardyB
	if err := json.NewDecoder(resp.Body).Decode(&cardyB); err != nil {
		return "", util.WrapErr("failed to decode cardyb response", err)
	}

	return cardyB.Title, nil
}
