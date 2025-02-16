package sites

import (
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"

	"github.com/bits-and-blooms/bloom/v3"
)

const EstimatedTotalEvents = 110000000 // 110 million. Estimate is used to create bloom filter used for duplicate detection.
const DuplicatePrecision = 0.001       // 0.1% precision for duplicate detection

type Aggregation struct {
	items   map[string]AggregationItem
	filter  *bloom.BloomFilter
	total   int // Number of events processed
	skipped int // Number of events skipped due to suspected duplicate
}

func NewAggregation() Aggregation {
	return Aggregation{
		items:   make(map[string]AggregationItem),
		filter:  bloom.NewWithEstimates(EstimatedTotalEvents, DuplicatePrecision),
		total:   0,
		skipped: 0,
	}
}

func (a *Aggregation) Get(host string) AggregationItem {
	return a.items[host]
}

func (a *Aggregation) Total() int {
	return a.total
}

func (a *Aggregation) Skipped() int {
	return a.skipped
}

func (a *Aggregation) CountEvent(eventType int, linkURL string, did string) {
	// Fetch the domain from the URL
	url, err := url.Parse(linkURL)
	if err != nil {
		slog.Warn("failed to parse url when counting event", "url", linkURL)
		return
	}
	host := url.Hostname()

	// Trim 'www.' prefix if it exists
	host = strings.TrimPrefix(host, "www.")

	if host == "" {
		slog.Warn("empty host when parsing url", "url", linkURL)
		return
	}

	// Use bloom filter to detect duplicates.
	fingerprint := fmt.Sprintf("%s%d%s", linkURL, eventType, did)
	if a.filter.TestAndAddString(fingerprint) {
		a.skipped++
		return
	}

	// Create aggregation item for host if it doesn't eixst
	if _, ok := a.items[host]; !ok {
		a.items[host] = AggregationItem{}
	}

	item := a.items[host]
	item.CountEvent(eventType, linkURL, did)
	a.items[host] = item

	a.total++
}

func (a *Aggregation) TopSites(n int) []string {
	// Convert map to slice
	type kv struct {
		Domain          string
		AggregationItem AggregationItem
	}

	var kvs []kv
	for k, v := range a.items {
		kvs = append(kvs, kv{Domain: k, AggregationItem: v})
	}

	// Sort by total interactions
	slices.SortFunc(kvs, func(a, b kv) int {
		scoreA := a.AggregationItem.counts.Total()
		scoreB := b.AggregationItem.counts.Total()

		if scoreA > scoreB {
			return -1
		}
		if scoreA < scoreB {
			return 1
		}
		return 0
	})

	// Find top n items
	sites := make([]string, 0, n)
	for i := range kvs {
		if len(sites) >= n {
			break
		}
		sites = append(sites, kvs[i].Domain)
	}

	return sites
}
