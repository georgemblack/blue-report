package app

import (
	"log/slog"
	"sync"
	"time"

	"github.com/georgemblack/blue-report/pkg/sites"
	"github.com/georgemblack/blue-report/pkg/storage"
	"github.com/georgemblack/blue-report/pkg/urltools"
	"github.com/georgemblack/blue-report/pkg/util"
)

const SiteAggregationWorkerCount = 6

// AggregateSites fetches all events from storage, aggregates top sites, and generates a snapshot.
// For each site, we aggregate the top URLs shared, total user interactions, and more.
func AggregateSites() (sites.Snapshot, error) {
	slog.Info("starting snapshot generation")
	jobStart := time.Now()

	app, err := NewApp()
	if err != nil {
		return sites.Snapshot{}, util.WrapErr("failed to create app", err)
	}
	defer app.Close()

	// Create the aggregation.
	// This will be used to generate all the data required to render the report.
	aggregation := sites.NewAggregation()
	end := time.Now().UTC()
	start := end.Add(-30 * 24 * time.Hour) // 30 days

	// Fetch all known translations (i.e. URL redirects).
	// Apply them as we process events.
	translations, err := app.Storage.GetURLTranslations()
	if err != nil {
		return sites.Snapshot{}, util.WrapErr("failed to get url translations", err)
	}
	slog.Info("loaded url translations", "count", len(translations))

	chunks, err := app.Storage.ListEventChunks(start, end)
	if err != nil {
		return sites.Snapshot{}, util.WrapErr("failed to list event chunks", err)
	}
	length := len(chunks)

	// Start worker threads to divide the work.
	// This way, when one is blocked via network, the other can continue processing.
	var wg sync.WaitGroup
	wg.Add(SiteAggregationWorkerCount)
	errs := make(chan error, SiteAggregationWorkerCount)

	// Divide the work into segments and start workers
	segmentSize := length / SiteAggregationWorkerCount
	for i := 0; i < SiteAggregationWorkerCount; i++ {
		start := i * segmentSize
		end := (i + 1) * segmentSize
		if i == SiteAggregationWorkerCount-1 {
			end = length
		}
		go aggregateSitesWorker(i, app.Storage, chunks[start:end], &aggregation, translations, &wg, errs)
	}

	wg.Wait()
	close(errs)

	// Check for any errors
	for err := range errs {
		if err != nil {
			return sites.Snapshot{}, util.WrapErr("failed to aggregate sites", err)
		}
	}

	slog.Info("processed events", "count", aggregation.Total(), "skipped", aggregation.Skipped())

	// Sort sites based on number of interactions
	top := aggregation.TopSites(ListSize)

	// Format data into a snapshot
	snapshot := sites.NewSnapshot()
	for _, site := range top {
		snapshot.AddSite(site, aggregation.Get(site))
	}

	// Hydrate the snapshot with metadata from storage
	snapshot, err = hydrateSites(app, &aggregation, snapshot)
	if err != nil {
		return sites.Snapshot{}, util.WrapErr("failed to hydrate sites", err)
	}

	jobDuration := time.Since(jobStart)
	slog.Info("aggregation complete", "seconds", jobDuration.Seconds())
	return snapshot, nil
}

func aggregateSitesWorker(id int, st Storage, chunks []string, agg *sites.Aggregation, trans map[string]string, wg *sync.WaitGroup, errs chan error) {
	defer wg.Done()

	if len(chunks) == 0 {
		return
	}

	// Prefetch the first chunk
	type fetchResult struct {
		records []storage.EventRecord
		err     error
	}
	nextResult := make(chan fetchResult, 1)
	go func() {
		records, err := st.ReadEvents(chunks[0], EventBufferSize)
		nextResult <- fetchResult{records, err}
	}()

	for i, chunk := range chunks {
		slog.Debug("processing chunk", "worker", id, "chunk", chunk)

		// Wait for the prefetched result
		result := <-nextResult
		if result.err != nil {
			errs <- util.WrapErr("failed to read events", result.err)
			return
		}

		// Start prefetching the next chunk while we process this one
		if i+1 < len(chunks) {
			nextChunk := chunks[i+1]
			go func() {
				records, err := st.ReadEvents(nextChunk, EventBufferSize)
				nextResult <- fetchResult{records, err}
			}()
		}

		for _, record := range result.records {
			// URLs stored in events should already be filtered and normalized.
			// However, as rules change, past events may need to be re-processed.
			// This ensures the most up-to-date rules are applied.
			// ProcessURL combines Ignore + Clean + host extraction in a single url.Parse.
			cleanedURL, host, ignore := urltools.ProcessURL(record.URL)
			if ignore {
				continue
			}

			// Determine if there is a known translation (i.e. redirect) for this URL.
			// If so, use the translated URL instead.
			if translated, ok := trans[cleanedURL]; ok {
				cleanedURL = translated
			}

			// Count the event. This is thread safe.
			agg.CountEvent(record.Type, cleanedURL, host, record.DID)
		}

		result.records = nil // Help the garbage collector
	}
}
