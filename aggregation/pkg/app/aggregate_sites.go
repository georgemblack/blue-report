package app

import (
	"time"

	"github.com/georgemblack/blue-report/pkg/app/util"
	"github.com/georgemblack/blue-report/pkg/sites"
	"github.com/georgemblack/blue-report/pkg/storage"
)

// AggregateLinks fetches all events from storage, aggregates top sites, and generates a snapshot.
// For each site, we aggregate the top URLs shared, total user interactions, and more.
func AggregateSites() (sites.Snapshot, error) {
	// Build storage client
	stg, err := storage.New()
	if err != nil {
		return sites.Snapshot{}, util.WrapErr("failed to create storage client", err)
	}

	processed := 0
	aggregation := sites.Aggregation{}
	end := time.Now().UTC()
	start := end.Add(-30 * 60 * time.Hour)

	// Scan all events within the last 30 days, and return a map of sites and their associated data.
	chunks, err := stg.ListEventChunks(start, end)
	if err != nil {
		return sites.Snapshot{}, util.WrapErr("failed to list event chunks", err)
	}

	for _, chunk := range chunks {
		records, err := stg.ReadEvents(chunk)
		if err != nil {
			return sites.Snapshot{}, util.WrapErr("failed to read events", err)
		}

		for _, record := range records {
			// URLs stored in events should already be normalized.
			// However, as normalization rules change, past events may not be normalized.
			// This ensures the most up-to-date rules are applied.
			normalizedURL := normalize(record.URL)

			// Count the event
			aggregation.CountEvent(record.Type, normalizedURL, record.DID)
		}

		processed += len(records)
		records = nil // Help the garbage collector
	}

	// Sort the sites by the total number of interactions.
	_ = aggregation.TopSites(10)

	return sites.Snapshot{}, nil
}
