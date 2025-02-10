package app

import (
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/georgemblack/blue-report/pkg/app/util"
	"github.com/georgemblack/blue-report/pkg/display"
)

// DEPRECATED
// Format the result of an aggregation into a report.
func formatReport(count map[string]URLAggregation) (Report, error) {
	// Convert each item in map to ReportItem
	converted := make([]ReportItem, 0, len(count))
	for k, v := range count {
		converted = append(converted, ReportItem{URL: k, Aggregation: v})
	}

	// Sort ReportItems by score
	sorted := converted
	slices.SortFunc(sorted, func(a, b ReportItem) int {
		scoreA := a.Aggregation.Score()
		scoreB := b.Aggregation.Score()

		if scoreA > scoreB {
			return -1
		}
		if scoreA < scoreB {
			return 1
		}
		return 0
	})

	// Find top N items
	items := make([]ReportItem, 0, ListSize)
	for i := range sorted {
		if len(items) >= ListSize {
			break
		}
		items = append(items, sorted[i])
	}

	// Assemble report
	return Report{
		Items: items,
	}, nil
}

// DEPRECATED
func hydrateReport(ch Cache, stg Storage, report Report) (Report, error) {
	var err error

	// Display in Eastern time, as this site is targeted at a US audience
	report.GeneratedAt = util.ToEastern(time.Now()).Format("Jan 2, 2006 at 3:04pm (MST)")

	// For each report item, fetch the URL record from the cache and populate
	for i := range report.Items {
		report.Items[i], err = hydrateReportItem(ch, stg, i, report.Items[i])
		if err != nil {
			return Report{}, util.WrapErr("failed to hydrate item", err)
		}
	}

	return report, nil
}

// DEPRECATED
// Hydrate a single report item with:
//   - Metadata from the cache (title)
//   - Thumbnail image from S3
//   - Nicely formatted strings for rendering the report template
func hydrateReportItem(ch Cache, stg Storage, index int, item ReportItem) (ReportItem, error) {
	hashedURL := util.Hash(item.URL)
	record, err := ch.ReadURL(hashedURL)
	if err != nil {
		return ReportItem{}, util.WrapErr("failed to read url record", err)
	}

	item.EscapedURL = url.QueryEscape(item.URL)

	// Fetch the thumbnail from the Bluesky CDN and store it in our S3 bucket.
	// The thumbnail ID is the hash of the URL.
	if record.ImageURL != "" {
		err := stg.SaveThumbnail(hashedURL, record.ImageURL)
		if err != nil {
			slog.Warn(util.WrapErr("failed to save thumbnail", err).Error(), "url", item.URL)
		}
	}

	// Set the thumbnail URL if it exists
	exists, err := stg.ThumbnailExists(hashedURL)
	if err != nil {
		slog.Warn(util.WrapErr("failed to check for thumbnail", err).Error(), "url", item.URL)
	} else if exists {
		item.ThumbnailURL = fmt.Sprintf("/thumbnails/%s.jpg", hashedURL)
	}

	// Set display items, such as title, host, and stats
	item.Title = record.Title
	if item.Title == "" {
		item.Title = "(No title)"
	}
	item.Host = strings.TrimPrefix(hostname(item.URL), "www.")
	item.Rank = index + 1

	item.Display.Posts = display.FormatCount(record.Totals.Posts)
	item.Display.Reposts = display.FormatCount(record.Totals.Reposts)
	item.Display.Likes = display.FormatCount(record.Totals.Likes)

	clicks := clicks(item.URL)
	item.Display.Clicks = display.FormatCount(clicks)

	slog.Debug("hydrated", "record", item)
	return item, nil
}
