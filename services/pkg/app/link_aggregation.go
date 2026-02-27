package app

import (
	"log/slog"
	"sync"
	"time"

	"github.com/georgemblack/blue-report/pkg/links"
	"github.com/georgemblack/blue-report/pkg/storage"
	"github.com/georgemblack/blue-report/pkg/urltools"
	"github.com/georgemblack/blue-report/pkg/util"
)

const (
	ListSize                   = 10
	LinkAggregationWorkerCount = 6
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

	// Create the time boundaries for the 'previous day' and 'previous week' reports.
	now := time.Now().UTC()
	bounds := links.TimeBounds{
		HourStart: now.Add(-1 * time.Hour),
		DayStart:  now.Add(-24 * time.Hour),
		WeekStart: now.Add(-24 * 7 * time.Hour),
	}

	// Create the aggregation.
	// This will be used to generate all the data required to render the report.
	aggregation := links.NewAggregation(bounds)

	// Fetch all known translations (i.e. URL redirects).
	// Apply them as we process events.
	translations, err := app.Storage.GetURLTranslations()
	if err != nil {
		return links.Snapshot{}, util.WrapErr("failed to get url translations", err)
	}
	slog.Info("loaded url translations", "count", len(translations))

	chunks, err := app.Storage.ListEventChunks(bounds.WeekStart, now)
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
	topHour := aggregation.TopHourLinks(ListSize)
	topDay := aggregation.TopDayLinks(ListSize)
	topWeek := aggregation.TopWeekLinks(ListSize)

	// Format the data into a snapshot
	snapshot := links.NewSnapshot()

	hour := make([]links.Link, 0, len(topHour))
	for _, url := range topHour {
		hour = append(hour, links.Link{
			URL: url,
		})
	}
	day := make([]links.Link, 0, len(topDay))
	for _, url := range topDay {
		day = append(day, links.Link{
			URL: url,
		})
	}
	week := make([]links.Link, 0, len(topWeek))
	for _, url := range topWeek {
		week = append(week, links.Link{
			URL: url,
		})
	}

	snapshot.TopHour = hour
	snapshot.TopDay = day
	snapshot.TopWeek = week

	// Hydrate the snapshot with metadata from storage, as well as the cache
	snapshot, err = hydrateLinks(app, &aggregation, snapshot)
	if err != nil {
		return links.Snapshot{}, util.WrapErr("failed to hydrate links", err)
	}

	jobDuration := time.Since(jobStart)
	slog.Info("aggregation complete", "seconds", jobDuration.Seconds())
	return snapshot, nil
}

func aggregateLinksWorker(id int, st Storage, chunks []string, agg *links.Aggregation, trans map[string]string, wg *sync.WaitGroup, errs chan error) {
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
			// ProcessURL combines Ignore + Clean in a single url.Parse.
			cleanedURL, _, ignore := urltools.ProcessURL(record.URL)
			if ignore {
				continue
			}

			// Determine if there is a known translation (i.e. redirect) for this URL.
			// If so, use the translated URL instead.
			if translated, ok := trans[cleanedURL]; ok {
				cleanedURL = translated
			}

			// Count the event. This is thread safe.
			agg.CountEvent(record.Type, cleanedURL, record.Post, record.DID, record.Timestamp)
		}

		result.records = nil // Help the garbage collector
	}
}
