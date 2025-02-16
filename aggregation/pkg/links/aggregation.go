package links

import (
	"fmt"
	"slices"

	"github.com/bits-and-blooms/bloom/v3"
	mapset "github.com/deckarep/golang-set/v2"
)

const EstimatedTotalEvents = 3000000 // 3 million. Estimate is used to create bloom filter used for duplicate detection.]
const DuplicatePrecision = 0.001     // 0.1% precision for duplicate detection

type Aggregation struct {
	items        map[string]AggregationItem
	filter       *bloom.BloomFilter
	fingerprints mapset.Set[string]
	total        int // Number of events processed
	skipped      int // Number of events skipped due to suspected duplicate
}

func NewAggregation() Aggregation {
	return Aggregation{
		items:   make(map[string]AggregationItem),
		filter:  bloom.NewWithEstimates(EstimatedTotalEvents, DuplicatePrecision),
		total:   0,
		skipped: 0,
	}
}

func (a *Aggregation) Get(url string) AggregationItem {
	return a.items[url]
}

func (a *Aggregation) Total() int {
	return a.total
}

func (a *Aggregation) Skipped() int {
	return a.skipped
}

func (a *Aggregation) CountEvent(eventType int, linkURL string, post string, did string) {
	fingerprint := fmt.Sprintf("%s%d%s", linkURL, eventType, did)
	if a.filter.TestAndAddString(fingerprint) {
		a.skipped++
		return
	}

	item := a.items[linkURL]
	item.CountEvent(eventType, post)
	a.items[linkURL] = item

	a.fingerprints.Add(fingerprint)
	a.total++
}

func (a *Aggregation) TopLinks(n int) []string {
	// Convert map to slice
	type kv struct {
		URL             string
		AggregationItem AggregationItem
	}
	var kvs []kv
	for k, v := range a.items {
		kvs = append(kvs, kv{URL: k, AggregationItem: v})
	}

	// Sort by score
	slices.SortFunc(kvs, func(a, b kv) int {
		scoreA := a.AggregationItem.Score()
		scoreB := b.AggregationItem.Score()

		if scoreA > scoreB {
			return -1
		}
		if scoreA < scoreB {
			return 1
		}
		return 0
	})

	// Find top N items
	urls := make([]string, 0, n)
	for i := range kvs {
		if len(urls) >= n {
			break
		}
		urls = append(urls, kvs[i].URL)
	}

	return urls
}
