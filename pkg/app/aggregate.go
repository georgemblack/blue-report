package app

import (
	"bytes"
	"embed"
	"fmt"
	"hash/fnv"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"text/template"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
)

//go:embed assets/index.html
var indexTemplate embed.FS

const (
	ListSize = 20
)

func Aggregate() error {
	slog.Info("starting aggregation")
	start := time.Now()

	// Build Valkey client
	client, err := cacheClient()
	if err != nil {
		return wrapErr("failed to create valkey client", err)
	}
	defer client.Close()

	keys, err := client.Keys()
	if err != nil {
		return wrapErr("failed to list keys", err)
	}
	slog.Info("found keys", "keys", keys.Cardinality())

	type CountItem struct {
		PostCount   int
		RepostCount int
	}
	count := make(map[string]CountItem)     // Track instances each URL is shared
	fingerprints := mapset.NewSet[string]() // Track unique DID and URL combinations

	// Read all records, and aggregate count for each URL.
	// Exclude unwated URLs, as well as duplicate URLs from the same user.
	for key := range keys.Iter() {
		record, err := client.Read(key)
		if err != nil {
			slog.Warn(wrapErr("failed to read record", err).Error())
			continue
		}

		if !record.Valid() {
			continue
		}

		if !include(record.URL) {
			continue
		}

		print := fingerprint(record)
		normalized := Normalize(record.URL)

		// Each URL is counted only once per user. This is to avoid users reposting/spamming their links.
		// Use a 'fingerprint' to track a given URL and user combination.
		if fingerprints.Contains(print) {
			continue
		}

		// Update count for the URL and add fingerprint to set
		item := count[normalized]
		if record.isPost() {
			item.PostCount++
		} else if record.isRepost() {
			item.RepostCount++
		}
		count[normalized] = item
		fingerprints.Add(print)
	}

	slog.Info("finished generating count", "urls", len(count))

	// Result represents a single item to be rendered on the webpage
	type Result struct {
		Rank             int
		URL              string
		Title            string
		ImageURL         string
		PlaceholderImage bool
		PostCount        int
		RepostCount      int
		PostCountStr     string
		RepostCountStr   string
	}

	// Convert the map containing the count to the results
	formatted := make([]Result, 0, len(count))
	for k, v := range count {
		formatted = append(formatted, Result{URL: k, PostCount: v.PostCount, RepostCount: v.RepostCount})
	}

	// Sort results, and only keep top number of items
	sorted := formatted
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].PostCount+sorted[i].RepostCount > sorted[j].PostCount+sorted[j].RepostCount
	})
	top := sorted[:ListSize]

	// Hydrate data for each result.
	//	- Fetch page title
	//	- Fetch page image
	//	- Supplement any missing data
	for i := range top {
		url := top[i].URL
		title, img, err := fetchURLMetadata(url)
		if err != nil {
			slog.Warn(wrapErr("failed to fetch url metadata", err).Error())
		}

		top[i].Rank = i + 1
		top[i].Title = title
		top[i].ImageURL = img
		top[i].PostCountStr = strconv.FormatInt(int64(top[i].PostCount), 10)
		top[i].RepostCountStr = strconv.FormatInt(int64(top[i].RepostCount), 10)

		if top[i].Title == "" {
			top[i].Title = top[i].URL
		}
		if top[i].ImageURL == "" {
			top[i].PlaceholderImage = true
		} else {
			top[i].PlaceholderImage = false
		}

		slog.Info("hydrated", "record", top[i])
	}

	tmpl, err := template.ParseFS(indexTemplate, "assets/index.html")
	if err != nil {
		return wrapErr("failed to parse template", err)
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, top)
	if err != nil {
		return wrapErr("failed to execute template", err)
	}

	// For local testing
	os.WriteFile("result.html", buf.Bytes(), 0644)

	// Publish report
	err = publish(buf.Bytes())
	if err != nil {
		return wrapErr("failed to publish report", err)
	}

	duration := time.Since(start)
	slog.Info("aggregation complete", "seconds", duration.Seconds())
	return nil
}

// Generate a unique 'fingerprint' for a given user (DID) and URL combination.
func fingerprint(record InternalRecord) string {
	input := fmt.Sprintf("%s%s", record.DID, record.URL)
	hasher := fnv.New64a()
	hasher.Write([]byte(input))
	return fmt.Sprintf("%x", hasher.Sum(nil))
}
