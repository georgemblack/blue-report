package app

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"hash/fnv"
	"log/slog"
	"os"
	"sort"
	"text/template"

	mapset "github.com/deckarep/golang-set/v2"
)

//go:embed assets/index.html
var indexTemplate embed.FS

const (
	ListSize = 10
)

func Aggregate() error {
	slog.Info("starting aggregation")

	// Build Valkey client
	client, err := valkeyClient()
	if err != nil {
		return wrapErr("failed to create valkey client", err)
	}
	defer client.Close()

	ctx := context.Background()
	cursor := uint64(0)
	first := true
	keys := mapset.NewSet[string]()

	// Collect a set of all keys in Valkey.
	for cursor != 0 || first {
		first = false

		cmd := client.B().Scan().Cursor(cursor).Build()
		resp := client.Do(ctx, cmd)
		if err := resp.Error(); err != nil {
			return wrapErr("failed to execute scan command", err)
		}

		// Valkey returns an array of two items: next cursor and a list of keys
		items, err := resp.ToArray()
		if err != nil {
			return wrapErr("failed to convert response to array", err)
		}
		if len(items) != 2 {
			return wrapErr("unexpected number of items in response", nil)
		}
		cursor, err = items[0].AsUint64()
		if err != nil {
			return wrapErr("failed to convert cursor to int64", err)
		}
		toAdd, err := items[1].AsStrSlice()
		if err != nil {
			return wrapErr("failed to convert keys to string slice", err)
		}

		for _, key := range toAdd {
			keys.Add(key)
		}
	}

	slog.Info("found keys", "count", keys.Cardinality())

	count := make(map[string]int)
	fingerprints := mapset.NewSet[string]()

	for key := range keys.Iter() {
		record, err := read(client, key)
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

	type Result struct {
		URL         string
		Title       string
		Description string
		ImageURL    string
		Count       int
	}

	// Convert map to slice of results
	formatted := make([]Result, 0, len(count))
	for k, v := range count {
		formatted = append(formatted, Result{URL: k, Count: v})
	}

	// Sort results by count
	sorted := formatted
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Count > sorted[j].Count
	})

	// Find top fifty links
	top := make([]Result, 0, ListSize)
	for i := 0; i < ListSize && i < len(sorted); i++ {
		top = append(top, sorted[i])
	}

	// Hydrate webpage metadata for top links
	for i := range top {
		url := formatted[i].URL
		title, desc, img, err := fetchOpenGraphData(url)
		if err != nil {
			slog.Warn(wrapErr("failed to fetch open graph data", err).Error())
			continue
		}

		top[i].Title = title
		top[i].Description = desc
		top[i].ImageURL = img
	}

	// Render webpage
	tmpl, err := template.ParseFS(indexTemplate, "assets/index.html")
	if err != nil {
		return wrapErr("failed to parse template", err)
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, top)
	if err != nil {
		return wrapErr("failed to execute template", err)
	}

	// Temporary: write to file
	os.WriteFile("result.html", buf.Bytes(), 0644)

	return nil
}

// Generate a unique 'fingerprint' for a given user (DID) and URL combination.
func fingerprint(record InternalRecord) string {
	input := fmt.Sprintf("%s%s", record.DID, record.URL)
	hasher := fnv.New64a()
	hasher.Write([]byte(input))
	return fmt.Sprintf("%x", hasher.Sum(nil))
}
