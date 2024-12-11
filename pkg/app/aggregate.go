package app

import (
	"bytes"
	"embed"
	"fmt"
	"hash/fnv"
	"log/slog"
	"os"
	"sort"
	"text/template"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"golang.org/x/text/message"
)

//go:embed assets/index.html
var indexTmpl embed.FS

const (
	ListSize     = 20
	PauseMinutes = 5
)

// Aggregate begins the aggregation loop.
// All records are read from Valkey, and the frequency of each URL is counted.
// An in-memory cache supplements Valkey, that persists between runs.
func Aggregate() error {
	cache := map[string]InternalCacheRecord{}

	for {
		err := aggregate(cache)
		if err != nil {
			return err
		}
	}
}

func aggregate(cache map[string]InternalCacheRecord) error {
	slog.Info("starting aggregation")
	start := time.Now()

	// Build Valkey client
	vk, err := valkeyClient()
	if err != nil {
		return wrapErr("failed to create valkey client", err)
	}
	defer vk.Close()

	keys, err := vk.Keys()
	if err != nil {
		return wrapErr("failed to list keys", err)
	}
	slog.Info("found keys", "keys", keys.Cardinality())

	count := make(map[string]Count)         // Track instances each URL is shared
	fingerprints := mapset.NewSet[string]() // Track unique DID and URL combinations

	// Read all records, and aggregate count for each URL.
	// Exclude unwated URLs, as well as duplicate URLs from the same user.
	internalCacheHit := 0
	externalCacheHit := 0

	for key := range keys.Iter() {
		record := EventRecord{}

		// Check internal cache for key
		hit, ok := cache[key]
		if ok {
			// If the record has expired or empty, delete from local cache and skip
			if hit.Expired() || hit.Record.Empty() {
				delete(cache, key)
				continue
			}

			record = hit.Record
			internalCacheHit++
		} else {
			// Read record from Valkey
			record, err = vk.Read(key)
			if err != nil {
				slog.Warn(wrapErr("failed to read record", err).Error())
				continue
			}

			// If the record in Valkey is expired, delete from local cache and skip
			if record.Empty() {
				delete(cache, key)
				continue
			}

			// Get record TTL from Valkey. This will be used to set our own internal cache's expiry.
			// If the internal cache's record is expired, we can assume the Valkey record is expired as well.
			ttl, err := vk.TTL(key)
			if err != nil {
				slog.Warn(wrapErr("failed to get ttl", err).Error())
				continue
			}

			cache[key] = InternalCacheRecord{
				Record: record,
				Expiry: time.Now().Add(time.Second * time.Duration(ttl)),
			}
			externalCacheHit++
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

	slog.Info("finished generating count", "urls", len(count), "internal_cache_hit", internalCacheHit, "external_cache_hit", externalCacheHit)

	// Convert the map containing the count to the results
	formatted := make([]ReportLinks, 0, len(count))
	for k, v := range count {
		formatted = append(formatted, ReportLinks{URL: k, PostCount: v.PostCount, RepostCount: v.RepostCount})
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
		top[i].Host = hostname(url)
		top[i].ImageURL = img
		if top[i].Title == "" {
			top[i].Title = top[i].URL
		}

		p := message.NewPrinter(message.MatchLanguage("en"))
		top[i].PostCountStr = p.Sprintf("%d", top[i].PostCount)
		top[i].RepostCountStr = p.Sprintf("%d", top[i].RepostCount)

		slog.Debug("hydrated", "record", top[i])
	}

	// Generate final report
	report := Report{Links: top, GeneratedAt: time.Now().Format(time.RFC3339)}

	// Convert to HTML
	tmpl, err := template.ParseFS(indexTmpl, "assets/index.html")
	if err != nil {
		return wrapErr("failed to parse template", err)
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, report)
	if err != nil {
		return wrapErr("failed to execute template", err)
	}

	// For local testing
	os.WriteFile("result.html", buf.Bytes(), 0644)

	// Publish to S3
	// err = publish(buf.Bytes())
	// if err != nil {
	// 	return wrapErr("failed to publish report", err)
	// }

	duration := time.Since(start)
	slog.Info("aggregation complete", "seconds", duration.Seconds())
	return nil
}

// Generate a unique 'fingerprint' for a given user (DID) and URL combination.
func fingerprint(record EventRecord) string {
	input := fmt.Sprintf("%s%s", record.DID, record.URL)
	hasher := fnv.New64a()
	hasher.Write([]byte(input))
	return fmt.Sprintf("%x", hasher.Sum(nil))
}
