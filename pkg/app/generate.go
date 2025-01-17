package app

import (
	"embed"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/georgemblack/blue-report/pkg/app/util"
	"github.com/georgemblack/blue-report/pkg/cache"
	"github.com/georgemblack/blue-report/pkg/storage"
	"golang.org/x/text/message"
)

//go:embed assets/index.html
var indexTmpl embed.FS

const (
	ListSize = 15
)

// Generate fetches all events from storage, aggregates trending URLs, and generates a final report.
// Metadata for each URL is hydrated from the cache, and thumbnails for each URL are stored in S3.
func Generate() (Report, error) {
	slog.Info("starting report generation")
	start := time.Now()

	// Build the cache client
	ch, err := cache.New()
	if err != nil {
		return Report{}, util.WrapErr("failed to create the cache client", err)
	}
	defer ch.Close()

	// Build storage client
	stg, err := storage.New()
	if err != nil {
		return Report{}, util.WrapErr("failed to create storage client", err)
	}

	// Run the count
	count, err := count(stg)
	if err != nil {
		return Report{}, util.WrapErr("failed to generate count", err)
	}

	// Format results
	formatted, err := format(count)
	if err != nil {
		return Report{}, util.WrapErr("failed to format results", err)
	}

	// Hydrate report with data from cache, i.e. titles, image URLs, and more for each report item.
	hydrated, err := hydrate(ch, stg, formatted)
	if err != nil {
		return Report{}, util.WrapErr("failed to hydrate report", err)
	}

	duration := time.Since(start)
	slog.Info("aggregation complete", "seconds", duration.Seconds())
	return hydrated, nil
}

// Scan all events within the last 24 hours, and return a map of URLs and their associated counts.
// Ignore duplicate URLs from the same user.
// Example count: { "https://example.com": { Posts: 1, Reposts: 0, Likes: 0 } }
func count(stg Storage) (map[string]Aggregation, error) {
	count := make(map[string]Aggregation)   // Track each instance of a URL being shared
	fingerprints := mapset.NewSet[string]() // Track unique DID, URL, and event type combinations
	events := 0                             // Track total events processed
	denied := 0                             // Track duplicate URLs from the same user

	// Scan all events within the last 24 hours
	end := time.Now().UTC()
	start := end.Add(-24 * time.Hour)

	// Records are stored in 'chunks', which are processed sequentially to limit memory usage
	chunks, err := stg.ListEventChunks(start, end)
	if err != nil {
		return nil, util.WrapErr("failed to list event chunks", err)
	}

	for _, chunk := range chunks {
		records, err := stg.ReadEvents(chunk)
		if err != nil {
			return nil, util.WrapErr("failed to read events", err)
		}

		for _, record := range records {
			print := fingerprint(record)
			if fingerprints.Contains(print) {
				denied++
				continue
			}

			// URLs stored in events should already be normalized.
			// However, as normalization rules change, past events may not be normalized.
			// This ensures the most up-to-date rules are applied.
			normalizedURL := normalize(record.URL)

			// Update count for the URL and add fingerprint to set
			item := count[normalizedURL]
			if record.IsPost() {
				item.Posts++
			} else if record.IsRepost() {
				item.Reposts++
			} else if record.IsLike() {
				item.Likes++
			}
			count[normalizedURL] = item
			fingerprints.Add(print)
		}

		events += len(records)
	}

	slog.Info("finished generating count", "chunks", len(chunks), "processed", events, "denied", denied, "urls", len(count))
	return count, nil
}

// Generate a unique 'fingerprint' for a given user (DID), URL, and event type combination.
func fingerprint(record storage.EventRecord) string {
	return util.Hash(fmt.Sprintf("%d%s%s", record.Type, record.DID, record.URL))
}

// Format the result of an aggregation into a report.
func format(count map[string]Aggregation) (Report, error) {
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

func hydrate(ch Cache, stg Storage, report Report) (Report, error) {
	var err error

	// Display in Eastern time, as this site is targeted at a US audience
	report.GeneratedAt = util.ToEastern(time.Now()).Format("Jan 2, 2006 at 3:04pm (MST)")

	// For each report item, fetch the URL record from the cache and populate
	for i := range report.Items {
		report.Items[i], err = hydrateItem(ch, stg, i, report.Items[i])
		if err != nil {
			return Report{}, util.WrapErr("failed to hydrate item", err)
		}
	}

	return report, nil
}

// Hydrate a single report item with:
//   - Metadata from the cache (title)
//   - Thumbnail image from S3
//   - Nicely formatted strings for rendering the report template
func hydrateItem(ch Cache, stg Storage, index int, item ReportItem) (ReportItem, error) {
	hashedURL := util.Hash(item.URL)
	record, err := ch.ReadURL(hashedURL)
	if err != nil {
		return ReportItem{}, util.WrapErr("failed to read url record", err)
	}

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

	item.Title = record.Title
	if item.Title == "" {
		item.Title = "(No title)"
	}

	item.Host = strings.TrimPrefix(hostname(item.URL), "www.")
	item.Rank = index + 1

	p := message.NewPrinter(message.MatchLanguage("en"))
	item.Display.Posts = p.Sprintf("%d", record.Totals.Posts)
	item.Display.Reposts = p.Sprintf("%d", record.Totals.Reposts)
	item.Display.Likes = p.Sprintf("%d", record.Totals.Likes)

	slog.Debug("hydrated", "record", item)
	return item, nil
}
