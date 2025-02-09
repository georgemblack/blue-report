package app

import (
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/georgemblack/blue-report/pkg/app/util"
	"github.com/georgemblack/blue-report/pkg/cache"
	"github.com/georgemblack/blue-report/pkg/display"
	"github.com/georgemblack/blue-report/pkg/storage"
)

//go:embed assets/index.html
var indexTmpl embed.FS

const (
	ListSize = 15
)

// Generate fetches all events from storage, aggregates trending URLs, and generates a final report.
// Metadata for each URL is hydrated from the cache, and thumbnails for each URL are stored in S3.
// DEPRECATED: Remove 'Report' from return values
func Generate() (Report, Snapshot, error) {
	slog.Info("starting report generation")
	start := time.Now()

	// Build the cache client
	ch, err := cache.New()
	if err != nil {
		return Report{}, Snapshot{}, util.WrapErr("failed to create the cache client", err)
	}
	defer ch.Close()

	// Build storage client
	stg, err := storage.New()
	if err != nil {
		return Report{}, Snapshot{}, util.WrapErr("failed to create storage client", err)
	}

	// Run the count
	count, err := count(stg)
	if err != nil {
		return Report{}, Snapshot{}, util.WrapErr("failed to generate count", err)
	}

	// Format results
	formatted, err := format(count)
	if err != nil {
		return Report{}, Snapshot{}, util.WrapErr("failed to format results", err)
	}

	// DEPRECATED
	// Hydrate report with data from cache, i.e. titles, image URLs, and more for each report item.
	hydratedReport, err := hydrateReport(ch, stg, formatted)
	if err != nil {
		return Report{}, Snapshot{}, util.WrapErr("failed to hydrate report", err)
	}

	// Hydrate report with data from cache, i.e. titles, image URLs, and more for each report item.
	hydrated, err := hydrate(ch, stg, formatted)
	if err != nil {
		return Report{}, Snapshot{}, util.WrapErr("failed to hydrate report", err)
	}

	duration := time.Since(start)
	slog.Info("aggregation complete", "seconds", duration.Seconds())
	return hydratedReport, hydrated, nil
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
		records = nil // Help the garbage collector
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

func hydrate(ch Cache, stg Storage, report Report) (Snapshot, error) {
	var snapshot Snapshot
	snapshot.GeneratedAt = time.Now().UTC().Format(time.RFC3339)

	// For each report item, fetch the URL record from the cache and populate
	snapshot.Links = make([]Link, 0, len(report.Items))
	for i := range report.Items {
		link := Link{
			URL: report.Items[i].URL,
		}
		link, err := hydrateLink(ch, stg, i, link)
		if err != nil {
			return Snapshot{}, util.WrapErr("failed to hydrate link", err)
		}
		snapshot.Links = append(snapshot.Links, link)
	}

	return snapshot, nil
}

// Hydrate a single report item with:
//   - Metadata from the cache (title)
//   - Thumbnail image from S3
//   - Nicely formatted strings for rendering the report template
//
// DEPRECATED
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

func hydrateLink(ch Cache, stg Storage, index int, link Link) (Link, error) {
	hashedURL := util.Hash(link.URL)
	record, err := ch.ReadURL(hashedURL)
	if err != nil {
		return Link{}, util.WrapErr("failed to read url record", err)
	}

	// Fetch the thumbnail from the Bluesky CDN and store it in our S3 bucket.
	// The thumbnail ID is the hash of the URL.
	if record.ImageURL != "" {
		err := stg.SaveThumbnail(hashedURL, record.ImageURL)
		if err != nil {
			slog.Warn(util.WrapErr("failed to save thumbnail", err).Error(), "url", link.URL)
		}
	}

	// Set the thumbnail ID if it exists
	exists, err := stg.ThumbnailExists(hashedURL)
	if err != nil {
		slog.Warn(util.WrapErr("failed to check for thumbnail", err).Error(), "url", link.URL)
	} else if exists {
		link.ThumbnailID = hashedURL
	}

	// Set display items, such as title, host, and stats
	link.Title = record.Title
	if link.Title == "" {
		link.Title = "(No title)"
	}
	link.Rank = index + 1

	link.Aggregation.Posts = record.Totals.Posts
	link.Aggregation.Reposts = record.Totals.Reposts
	link.Aggregation.Likes = record.Totals.Likes
	link.Aggregation.Clicks = clicks(link.URL)

	slog.Debug("hydrated", "record", link)
	return link, nil
}

// Get the number of clicks for a given URL.
// This data is provided by The Blue Report API.
func clicks(url string) int {
	resp, err := http.Get(fmt.Sprintf("https://api.theblue.report?url=%s", url))
	if err != nil {
		err = util.WrapErr("failed to get clicks", err)
		slog.Error(err.Error(), "url", url)
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("failed to get clicks", "url", url, "status", resp.StatusCode)
		return 0
	}

	var result struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		err = util.WrapErr("failed to decode clicks response", err)
		slog.Error(err.Error(), "url", url)
		return 0
	}

	return result.Count
}
