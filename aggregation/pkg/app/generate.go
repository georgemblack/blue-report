package app

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/georgemblack/blue-report/pkg/app/util"
	"github.com/georgemblack/blue-report/pkg/bluesky"
	"github.com/georgemblack/blue-report/pkg/cache"
	"github.com/georgemblack/blue-report/pkg/storage"
)

const (
	ListSize = 15
)

// Generate fetches all events from storage, aggregates trending URLs, and generates a final report.
// Metadata for each URL is hydrated from the cache, and thumbnails for each URL are stored in S3.
func Generate() (Snapshot, error) {
	slog.Info("starting report generation")
	start := time.Now()

	// Build the cache client
	ch, err := cache.New()
	if err != nil {
		return Snapshot{}, util.WrapErr("failed to create the cache client", err)
	}
	defer ch.Close()

	// Build storage client
	stg, err := storage.New()
	if err != nil {
		return Snapshot{}, util.WrapErr("failed to create storage client", err)
	}

	// Build Bluesky client
	bs := bluesky.New()

	// Run the aggregation process, collecting data on each URL.
	// Collect data in a map of 'URL' -> 'URLAggregation'.
	aggregation, err := aggregate(stg)
	if err != nil {
		return Snapshot{}, util.WrapErr("failed to generate count", err)
	}

	// Find the top URLs based on score
	top := topURLs(aggregation)

	// Build an empty snapshot
	snapshot := newSnapshot(top)

	// Hydrate each item in the snapshot with data from the cache & storage
	snapshot, err = hydrate(ch, stg, bs, aggregation, snapshot)
	if err != nil {
		return Snapshot{}, util.WrapErr("failed to hydrate snapshot", err)
	}

	duration := time.Since(start)
	slog.Info("aggregation complete", "seconds", duration.Seconds())
	return snapshot, nil
}

// Scan all events within the last 24 hours, and return a map of URLs and their associated counts.
// Ignore duplicate URLs from the same user.
// Example aggregate: { "https://example.com": { Posts: 1, Reposts: 0, Likes: 0 } }
func aggregate(stg Storage) (Aggregation, error) {
	count := make(Aggregation)              // Track each instance of a URL being shared
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

			// Update post/respot/like count for the URL
			agg := count[normalizedURL]
			if record.IsPost() {
				agg.IncrementPostCount()
			} else if record.IsRepost() {
				agg.IncrementRepostCount()
			} else if record.IsLike() {
				agg.IncrementLikeCount()
			}

			// Update the set of posts referencing the URL
			agg.CountPost(record.Post)

			// Set the new aggregation, and update our set of fingerprints
			count[normalizedURL] = agg
			fingerprints.Add(print)
		}

		events += len(records)
		records = nil // Help the garbage collector
	}

	slog.Info("finished generating count", "chunks", len(chunks), "processed", events, "denied", denied, "urls", len(count))
	return count, nil
}

func topURLs(agg Aggregation) []string {
	// Convert map to slice
	type kv struct {
		URL         string
		Aggregation URLAggregation
	}
	var kvs []kv
	for k, v := range agg {
		kvs = append(kvs, kv{URL: k, Aggregation: v})
	}

	// Sort by score
	slices.SortFunc(kvs, func(a, b kv) int {
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
	urls := make([]string, 0, ListSize)
	for i := range kvs {
		if len(urls) >= ListSize {
			break
		}
		urls = append(urls, kvs[i].URL)
	}

	return urls
}

// Generate a unique 'fingerprint' for a given user (DID), URL, and event type combination.
func fingerprint(record storage.EventRecord) string {
	return util.Hash(fmt.Sprintf("%d%s%s", record.Type, record.DID, record.URL))
}

func hydrate(ch Cache, stg Storage, bs Bluesky, agg Aggregation, snapshot Snapshot) (Snapshot, error) {
	for i := range snapshot.Links {
		link, err := hydrateLink(ch, stg, bs, agg, i, snapshot.Links[i])
		if err != nil {
			return Snapshot{}, util.WrapErr("failed to hydrate link", err)
		}
		snapshot.Links[i] = link
	}

	return snapshot, nil
}

func hydrateLink(ch Cache, stg Storage, bs Bluesky, agg Aggregation, index int, link Link) (Link, error) {
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

	// Set display items, such as rank, title, host, and stats
	link.Rank = index + 1
	link.Title = record.Title
	if link.Title == "" {
		link.Title = "(No title)"
	}
	link.PostCount = record.Totals.Posts
	link.RepostCount = record.Totals.Reposts
	link.LikeCount = record.Totals.Likes
	link.ClickCount = clicks(link.URL)

	// Generate a list of the most popular 2-5 posts referencing the URL.
	// Posts should contain commentary on the subject of the link.
	aggregationItem := agg[link.URL]
	link.RecommendedPosts = recommendedPosts(bs, aggregationItem.TopPosts())

	slog.Debug("hydrated", "record", link)
	return link, nil
}

func recommendedPosts(bs Bluesky, uris []string) []Post {
	posts := make([]Post, 0)

	// For each AT URI, fetch the post from the Bluesky API.
	// If the post has enough text/commentary, add it to the list of recommended posts.
	for _, uri := range uris {
		if len(posts) >= 5 {
			break
		}

		postData, err := bs.GetPost(uri)
		if err != nil {
			slog.Warn(util.WrapErr("failed to get post", err).Error(), "at_uri", uri)
			continue
		}

		// In order for the post to be recommended, it must:
		//   - Be greater than 32 characters in length (to avoid posts that only contain the link)
		//   - Be in English (until there's multi language/region support)
		//   - Must have at least 50 likes (to avoid spam)
		if len(postData.Record.Text) >= 32 && postData.IsEnglish() && postData.LikeCount > 50 {
			posts = append(posts, Post{
				AtURI:    uri,
				Username: postData.Author.DisplayName,
				Handle:   postData.Author.Handle,
				Text:     postData.Record.Text,
			})
		}
	}

	return posts
}

// Get the number of clicks for a given URL.
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
