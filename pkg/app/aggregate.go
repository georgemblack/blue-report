package app

import (
	"bytes"
	"embed"
	"fmt"
	"hash/fnv"
	"log/slog"
	"sort"
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
	slog.Info("found keys", "count", keys.Cardinality())

	count := make(map[string]int)           // Track instances each URL is shared
	fingerprints := mapset.NewSet[string]() // Track unique DID and URL combinations

	// Read all records, and aggregate count for each URL.
	// Exclude unwated URLs, as well as duplicate URLs from the same user.
	for key := range keys.Iter() {
		record, err := client.Read(key)
		if err != nil {
			slog.Warn(wrapErr("failed to read record", err).Error())
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

		count[normalized]++
		fingerprints.Add(print)
	}

	// Result represents a single item to be rendered on the webpage
	type Result struct {
		Rank             int
		URL              string
		Title            string
		Description      string
		ImageURL         string
		PlaceholderImage bool
		Count            int
	}

	// Convert the map containing the count to the results
	formatted := make([]Result, 0, len(count))
	for k, v := range count {
		formatted = append(formatted, Result{URL: k, Count: v})
	}

	// Sort results, and only keep top number of items
	sorted := formatted
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Count > sorted[j].Count
	})
	top := sorted[:ListSize]

	// Hydrate data for each result.
	//	- Fetch page title
	//	- Fetch page image
	//	- Supplement any missing data
	for i := range top {
		slog.Info("hydrating result", "url", top[i].URL)

		url := top[i].URL
		title, desc, img, err := fetchURLMetadata(url)
		if err != nil {
			slog.Warn(wrapErr("failed to fetch open graph data", err).Error())
			continue
		}

		top[i].Title = title
		top[i].Description = desc
		top[i].ImageURL = img

		if top[i].Title == "" {
			top[i].Title = top[i].URL
		}
		if top[i].ImageURL == "" {
			top[i].PlaceholderImage = true
		} else {
			top[i].PlaceholderImage = false
		}
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
	// os.WriteFile("result.html", buf.Bytes(), 0644)

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
