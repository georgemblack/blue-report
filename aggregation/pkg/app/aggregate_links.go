package app

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/georgemblack/blue-report/pkg/storage"
	"github.com/georgemblack/blue-report/pkg/util"
)

const (
	ListSize = 15
)

// AggregateLinks fetches all events from storage, aggregates trending URLs, and generates a snapshot.
// Metadata for each URL is hydrated from the cache, and thumbnails for each URL are stored in S3.
func AggregateLinks() (LinkSnapshot, error) {
	slog.Info("starting snapshot generation")
	jobStart := time.Now()

	app, err := NewApp()
	if err != nil {
		return LinkSnapshot{}, util.WrapErr("failed to create app", err)
	}
	defer app.Close()

	// Run the aggregation process, collecting data on each URL.
	// Collect data in a map of 'URL' -> 'URLAggregation'.
	aggregation, err := aggregate(app.Storage)
	if err != nil {
		return LinkSnapshot{}, util.WrapErr("failed to generate count", err)
	}

	// Find the top URLs based on score
	top := topURLs(aggregation)

	// Build an empty snapshot
	snapshot := newLinkSnapshot(top)

	// Hydrate each item in the snapshot with data from the cache & storage
	snapshot, err = hydrate(app, aggregation, snapshot)
	if err != nil {
		return LinkSnapshot{}, util.WrapErr("failed to hydrate snapshot", err)
	}

	jobDuration := time.Since(jobStart)
	slog.Info("aggregation complete", "seconds", jobDuration.Seconds())
	return snapshot, nil
}

// Scan all events within the last 24 hours, and return a map of URLs and their associated counts.
// Ignore duplicate URLs from the same user.
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

// Given the AT URIs of the top posts referencing a URL, return a list of recommended posts to display to the user.
func recommendedPosts(bs Bluesky, uris []string) []Post {
	posts := make([]Post, 0)
	authors := mapset.NewSet[string]() // Track authors that already have a reocommended post

	// For each AT URI, fetch the post from the Bluesky API.
	// If the post has enough text/commentary, add it to the list of recommended posts.
	for _, uri := range uris {
		// Avoid fetching data after three posts have been selected
		if len(posts) >= 3 {
			break
		}

		postData, err := bs.GetPost(uri)
		if err != nil {
			slog.Warn(util.WrapErr("failed to get post", err).Error(), "at_uri", uri)
			continue
		}

		// In order for the post to be recommended:
		//   - Post must be >32 characters in length (to avoid posts that only contain the link)
		//   - Post must be in English (until there's multi language/region support)
		//   - Post must have at >=50 likes (to avoid spam)
		//	 - Post cannot be from an author who already has a recommended post for this link
		if len(postData.Record.Text) >= 32 && postData.IsEnglish() && postData.LikeCount > 50 && !authors.Contains(postData.Author.Handle) {
			posts = append(posts, Post{
				AtURI:    uri,
				Username: postData.Author.DisplayName,
				Handle:   postData.Author.Handle,
				Text:     postData.Record.Text,
			})
			authors.Add(postData.Author.Handle)
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
