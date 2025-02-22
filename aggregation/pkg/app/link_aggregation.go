package app

import (
	"log/slog"
	"sync"
	"time"

	"github.com/georgemblack/blue-report/pkg/links"
	"github.com/georgemblack/blue-report/pkg/urltools"
	"github.com/georgemblack/blue-report/pkg/util"
)

const (
	ListSize                   = 15
	LinkAggregationWorkerCount = 4
)

// AggregateLinks fetches all events from storage, aggregates trending URLs, and generates a snapshot.
// Metadata for each URL is hydrated from the cache, and thumbnails for each URL are stored in S3.
func AggregateLinks() (links.Snapshot, error) {
	slog.Info("starting snapshot generation")
	jobStart := time.Now()

	app, err := NewApp()
	if err != nil {
		return links.Snapshot{}, util.WrapErr("failed to create app", err)
	}
	defer app.Close()

	aggregation := links.NewAggregation()
	end := time.Now().UTC()
	start := end.Add(-24 * time.Hour) // 24 hours

	translations, err := app.Storage.GetURLTranslations()
	if err != nil {
		return links.Snapshot{}, util.WrapErr("failed to get url translations", err)
	}
	slog.Info("loaded translations", "count", len(translations))

	chunks, err := app.Storage.ListEventChunks(start, end)
	if err != nil {
		return links.Snapshot{}, util.WrapErr("failed to list event chunks", err)
	}
	length := len(chunks)

	// Start worker threads to divide the work.
	// This way, when one is blocked via network, the other can continue processing.
	var wg sync.WaitGroup
	wg.Add(LinkAggregationWorkerCount)
	errs := make(chan error, LinkAggregationWorkerCount)

	// Divide the work into segments and start workers
	segmentSize := length / LinkAggregationWorkerCount
	for i := 0; i < LinkAggregationWorkerCount; i++ {
		start := i * segmentSize
		end := (i + 1) * segmentSize
		if i == LinkAggregationWorkerCount-1 {
			end = length
		}
		go aggregateLinksWorker(i, app.Storage, chunks[start:end], &aggregation, translations, &wg, errs)
	}

	wg.Wait()
	close(errs)

	// Check for any errors
	for err := range errs {
		if err != nil {
			return links.Snapshot{}, util.WrapErr("failed to aggregate sites", err)
		}
	}

	slog.Info("processed events", "count", aggregation.Total(), "skipped", aggregation.Skipped())

	// Sort links based on score
	top := aggregation.TopLinks(ListSize)

	// Format the data into a snapshot
	snapshot := links.NewSnapshot()
	arr := make([]links.Link, 0, len(top))
	for _, url := range top {
		arr = append(arr, links.Link{
			URL: url,
		})
	}
	snapshot.Links = arr

	// Hydrate the snapshot with metadata from storage, as well as the cache
	snapshot, err = hydrateLinks(app, aggregation, snapshot)
	if err != nil {
		return links.Snapshot{}, util.WrapErr("failed to hydrate links", err)
	}

	jobDuration := time.Since(jobStart)
	slog.Info("aggregation complete", "seconds", jobDuration.Seconds())
	return snapshot, nil
}

func aggregateLinksWorker(id int, st Storage, chunks []string, agg *links.Aggregation, trans map[string]string, wg *sync.WaitGroup, errs chan error) {
	defer wg.Done()

	for _, chunk := range chunks {
		slog.Debug("processing chunk", "worker", id, "chunk", chunk)

		records, err := st.ReadEvents(chunk, EventBufferSize)
		if err != nil {
			errs <- util.WrapErr("failed to read events", err)
			return
		}

		for _, record := range records {
			// URLs stored in events should already be filtered and normalized.
			// However, as rules change, past events may need to be re-processed.
			// This ensures the most up-to-date rules are applied.
			if urltools.Ignore(record.URL) {
				continue
			}
			cleanedURL := urltools.Clean(record.URL)

			// Determine if there is a known translation (i.e. redirect) for this URL.
			// If so, use the translated URL instead.
			if translated, ok := trans[cleanedURL]; ok {
				cleanedURL = translated
			}

			// Count the event. This is thread safe.
			agg.CountEvent(record.Type, cleanedURL, record.Post, record.DID)
		}

		records = nil // Help the garbage collector
	}
}
