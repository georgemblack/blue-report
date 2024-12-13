package app

import (
	"bytes"
	"embed"
	"fmt"
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
	ListSize = 50
)

// Aggregate runs a single aggregation cycle and exits.
// All records are read from Valkey, and the frequency of each URL is counted.
// An in-memory cache supplements Valkey, that persists between runs.
func Aggregate() error {
	slog.Info("starting aggregation")
	start := time.Now()

	// Build Valkey client
	vk, err := NewValkeyClient()
	if err != nil {
		return wrapErr("failed to create valkey client", err)
	}
	defer vk.Close()

	// Build storage client
	stg, err := NewStorageClient()
	if err != nil {
		return wrapErr("failed to create storage client", err)
	}

	slog.Info("populating event cache")
	eventCache := NewEventCache()
	cacheDump, err := stg.ReadCache()
	if err != nil {
		slog.Warn(wrapErr("cache is starting empty", err).Error())
	} else {
		eventCache.Populate(cacheDump)
		slog.Info("cache populated", "items", len(cacheDump.Items))
	}

	// Find all event keys
	keys, err := vk.EventKeys()
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
		hit, ok := eventCache.Get(key)
		if ok {
			// If the record has expired or empty, delete from local cache and skip
			if hit.Expired() || hit.Record.Empty() {
				eventCache.Delete(key)
				continue
			}

			record = hit.Record
			internalCacheHit++
		} else {
			// Read record from Valkey
			record, err = vk.ReadEvent(key)
			if err != nil {
				slog.Warn(wrapErr("failed to read record", err).Error())
				continue
			}

			// If the record in Valkey is expired, delete from local cache and skip
			if record.Empty() {
				eventCache.Delete(key)
				continue
			}

			// Get record TTL from Valkey. This will be used to set our own internal cache's expiry.
			// If the internal cache's record is expired, we can assume the Valkey record is expired as well.
			ttl, err := vk.EventTTL(key)
			if err != nil {
				slog.Warn(wrapErr("failed to get ttl", err).Error())
				continue
			}

			eventCache.Add(key, CacheRecord{
				Record: record,
				Expiry: time.Now().Add(time.Second * time.Duration(ttl)),
			})
			externalCacheHit++
		}

		// Each URL is counted only once per user. This is to avoid users reposting/spamming their links.
		// Use a 'fingerprint' to track a given URL and user combination.
		print := fingerprint(record)
		if fingerprints.Contains(print) {
			continue
		}

		// Update count for the URL and add fingerprint to set
		item := count[record.URLHash]
		if record.isPost() {
			item.PostCount++
		} else if record.isRepost() {
			item.RepostCount++
		}
		count[record.URLHash] = item
		fingerprints.Add(print)
	}

	slog.Info("finished generating count", "urls", len(count), "internal_cache_hit", internalCacheHit, "external_cache_hit", externalCacheHit, "internal_cache_size", eventCache.Len())

	formatted := make([]ReportItems, 0, len(count))
	for k, v := range count {
		formatted = append(formatted, ReportItems{URLHash: k, Count: v})
	}

	// Sort results by score, and keep the top N
	sorted := formatted
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Count.Score() > sorted[j].Count.Score()
	})
	top := sorted[:ListSize]

	// Hydreate results with data for rendering webpage.
	// i.e. Title, description, host, etc.
	for i := range top {
		// Fetch record for URL
		urlRecord, err := vk.ReadURL(top[i].URLHash)
		if err != nil {
			return wrapErr("failed to read url record during hydration", err)
		}

		top[i].Rank = i + 1
		top[i].URL = urlRecord.URL
		top[i].Host = hostname(urlRecord.URL)
		top[i].Title = urlRecord.Title
		top[i].Description = urlRecord.Description
		top[i].ImageURL = urlRecord.ImageURL
		p := message.NewPrinter(message.MatchLanguage("en"))
		top[i].PostCountStr = p.Sprintf("%d", top[i].Count.PostCount)
		top[i].RepostCountStr = p.Sprintf("%d", top[i].Count.RepostCount)

		slog.Debug("hydrated", "record", top[i])
	}

	// Generate final report
	generatedAt := time.Now().Format("Jan 2, 2006 at 3:04pm (MST)")
	report := Report{Links: top, GeneratedAt: generatedAt}

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
	err = stg.PublishSite(buf.Bytes())
	if err != nil {
		return wrapErr("failed to publish report", err)
	}

	// Write the contents of the local cache to S3.
	// This wil be loaded on the next run, to avoid re-reading all records from Valkey.
	slog.Info("writing cache")
	cacheDump = eventCache.Dump()
	err = stg.WriteCache(cacheDump)
	if err != nil {
		slog.Warn(wrapErr("failed to write cache", err).Error())
	} else {
		slog.Info("cache written", "items", len(cacheDump.Items))
	}

	duration := time.Since(start)
	slog.Info("aggregation complete", "seconds", duration.Seconds())
	return nil
}

// Generate a unique 'fingerprint' for a given user (DID) and URL combination.
func fingerprint(record EventRecord) string {
	return hash(fmt.Sprintf("%s%s", record.DID, record.URLHash))
}
