package app

import (
	"bytes"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"sort"
	"text/template"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/georgemblack/blue-report/pkg/app/util"
	"github.com/georgemblack/blue-report/pkg/cache"
	"github.com/georgemblack/blue-report/pkg/storage"
	minify "github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"
	"golang.org/x/text/message"
)

//go:embed assets/index.html
var indexTmpl embed.FS

const (
	ListSize = 15
)

// Aggregate runs a single aggregation cycle and exits.
// All records are read from the cache, and the frequency of each URL is counted.
// An in-memory cache supplements the cache, that persists between runs.
func Aggregate() error {
	slog.Info("starting aggregation")
	start := time.Now()

	// Build the cache client
	ch, err := cache.New()
	if err != nil {
		return util.WrapErr("failed to create the cache client", err)
	}
	defer ch.Close()

	// Build storage client
	stg, err := storage.New()
	if err != nil {
		return util.WrapErr("failed to create storage client", err)
	}

	// Read all records from storage in chunks, and aggregate a count for each URL along the way.
	// Exclude unwated URLs, as well as duplicate URLs from the same user.
	endTime := time.Now().UTC()
	startTime := endTime.Add(-24 * time.Hour)

	count := make(map[string]Count)         // Track instances each URL is shared
	fingerprints := mapset.NewSet[string]() // Track unique DID and URL combinations
	events := 0                             // Track total events processed
	denied := 0                             // Track duplicate URLs from the same user

	chunks, err := stg.ListEventChunks(startTime, endTime)
	if err != nil {
		return util.WrapErr("failed to list event chunks", err)
	}

	for _, chunk := range chunks {
		records, err := stg.ReadEvents(chunk)
		if err != nil {
			return util.WrapErr("failed to read events", err)
		}

		for _, record := range records {
			print := fingerprint(record)
			if fingerprints.Contains(print) {
				denied++
				continue
			}

			// Update count for the URL and add fingerprint to set
			item := count[record.URL]
			if record.IsPost() {
				item.PostCount++
			} else if record.IsRepost() {
				item.RepostCount++
			} else if record.IsLike() {
				item.LikeCount++
			}
			count[record.URL] = item
			fingerprints.Add(print)
		}

		events += len(records)
	}

	slog.Info("finished generating count", "chunks", len(chunks), "processed", events, "denied", denied, "urls", len(count))

	formatted := make([]ReportItems, 0, len(count))
	for k, v := range count {
		formatted = append(formatted, ReportItems{URL: k, Count: v})
	}

	// Sort results by score, and keep the top N
	sorted := formatted
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Count.Score() > sorted[j].Count.Score()
	})

	// Generate lists top N lists by category, i.e. 'news', 'everything'
	news := make([]ReportItems, 0, ListSize)
	everything := make([]ReportItems, 0, ListSize)

	newsHosts, err := GetNewsHosts()
	if err != nil {
		return util.WrapErr("failed to get news hosts", err)
	}

	// Hydreate results with data for rendering webpage (i.e. title, description).
	// Place into 'news' or 'everything' list, and stop once both lists are full.
	for i := range sorted {
		// Fetch record for URL
		// TODO: Skip this call if the URL isn't going to be placed in existing lists
		urlRecord, err := ch.ReadURL(util.Hash(sorted[i].URL))
		if err != nil {
			return util.WrapErr("failed to read url record during hydration", err)
		}

		sorted[i].URL = urlRecord.URL
		sorted[i].Host = hostname(urlRecord.URL)
		sorted[i].Title = urlRecord.Title
		sorted[i].ImageURL = urlRecord.ImageURL
		p := message.NewPrinter(message.MatchLanguage("en"))
		sorted[i].PostCountStr = p.Sprintf("%d", sorted[i].Count.PostCount)
		sorted[i].RepostCountStr = p.Sprintf("%d", sorted[i].Count.RepostCount)
		sorted[i].LikeCountStr = p.Sprintf("%d", sorted[i].Count.LikeCount)

		slog.Debug("hydrated", "record", sorted[i])

		host := hostname(urlRecord.URL)
		if len(news) < ListSize && newsHosts.Contains(host) {
			news = append(news, sorted[i])
		}
		if len(everything) < ListSize && !newsHosts.Contains(host) {
			everything = append(everything, sorted[i])
		}
		if len(news) >= ListSize && len(everything) >= ListSize {
			break // Avoid hydrating more records than needed
		}
	}

	// Hydrate each set of results with ranks
	for i := range news {
		news[i].Rank = i + 1
	}
	for i := range everything {
		everything[i].Rank = i + 1
	}

	// Generate final report
	generatedAt := time.Now().Format("Jan 2, 2006 at 3:04pm (MST)")
	report := Report{NewsItems: news, EverythingItems: everything, GeneratedAt: generatedAt}

	// Convert to HTML
	tmpl, err := template.ParseFS(indexTmpl, "assets/index.html")
	if err != nil {
		return util.WrapErr("failed to parse template", err)
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, report)
	if err != nil {
		return util.WrapErr("failed to execute template", err)
	}

	// Minify HTML
	minifier := minify.New()
	minifier.Add("text/html", &html.Minifier{
		KeepDefaultAttrVals: true,
		KeepDocumentTags:    true,
		KeepEndTags:         true,
		KeepQuotes:          true,
	})
	minifier.AddFunc("text/css", css.Minify)
	minifier.AddFuncRegexp(regexp.MustCompile("^(application|text)/(x-)?(java|ecma)script$"), js.Minify)

	final, err := minifier.Bytes("text/html", buf.Bytes())
	if err != nil {
		return util.WrapErr("failed to minify html", err)
	}

	// For local testing
	os.WriteFile("result.html", final, 0644)

	// Publish to S3
	err = stg.PublishSite(final)
	if err != nil {
		return util.WrapErr("failed to publish report", err)
	}

	duration := time.Since(start)
	slog.Info("aggregation complete", "seconds", duration.Seconds())
	return nil
}

// Generate a unique 'fingerprint' for a given user (DID) and URL combination.
func fingerprint(record storage.EventRecord) string {
	return util.Hash(fmt.Sprintf("%d%s%s", record.Type, record.DID, record.URL))
}
