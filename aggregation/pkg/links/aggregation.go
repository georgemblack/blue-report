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
	correct      int // Track accuracy of the bloom filter
	incorrect    int // Track accuracy of the bloom filter
}

func NewAggregation() Aggregation {
	return Aggregation{
		items:        make(map[string]AggregationItem),
		filter:       bloom.NewWithEstimates(EstimatedTotalEvents, DuplicatePrecision),
		fingerprints: mapset.NewSet[string](),
		total:        0,
		correct:      0,
		incorrect:    0,
		skipped:      0,
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

func (a *Aggregation) BloomFilterCorrect() int {
	return a.correct
}

func (a *Aggregation) BloomFilterIncorrect() int {
	return a.incorrect
}

func (a *Aggregation) CountEvent(eventType int, linkURL string, post string, did string) {
	// Add event fingerprint to set to prevent duplicates.
	// Compare results from set-based de-duplication to bloom filter-based de-duplication.
	setDuplicate := false
	bloomDuplicate := false
	fingerprint := fmt.Sprintf("%s%d%s", linkURL, eventType, did)
	if a.fingerprints.Contains(fingerprint) {
		setDuplicate = true
	}
	if a.filter.TestAndAddString(fingerprint) {
		bloomDuplicate = true
	}
	// If there's a mismatch between the set and bloom filter, record it.
	if setDuplicate == bloomDuplicate {
		a.correct++
	} else {
		a.incorrect++
	}
	if setDuplicate {
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
