package links

import (
	"fmt"
	"hash/fnv"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bits-and-blooms/bloom/v3"
)

const EstimatedTotalEvents = 25000000 // 25 million. Estimate is used to create bloom filter used for duplicate detection.
const DuplicatePrecision = 0.001      // 0.1% precision for duplicate detection
const NumShards = 1024                // Number of shards to use for parallel processing

type Aggregation struct {
	shards           []Shard
	fingerprints     *bloom.BloomFilter
	fingerprintsLock sync.Mutex
	bounds           TimeBounds
	total            int64 // Number of events processed
	skipped          int64 // Number of events skipped due to suspected duplicate
}

type Shard struct {
	lock  sync.Mutex
	items map[string]*AggregationItem
}

type TimeBounds struct {
	DayStart  time.Time // Start of the 'previous day' report
	WeekStart time.Time // Start of the 'previous week' report
}

func NewAggregation(bounds TimeBounds) Aggregation {
	shards := make([]Shard, NumShards)
	for i := range shards {
		shards[i] = Shard{
			lock:  sync.Mutex{},
			items: make(map[string]*AggregationItem),
		}
	}

	return Aggregation{
		shards:           shards,
		fingerprints:     bloom.NewWithEstimates(EstimatedTotalEvents, DuplicatePrecision),
		fingerprintsLock: sync.Mutex{},
		bounds:           bounds,
		total:            0,
		skipped:          0,
	}
}

func (a *Aggregation) Get(url string) AggregationItem {
	shard := a.getShard(url)
	result := shard.items[url]
	if result == nil {
		return AggregationItem{}
	}
	return *result
}

func (a *Aggregation) Total() int64 {
	return a.total
}

func (a *Aggregation) Skipped() int64 {
	return a.skipped
}

func (a *Aggregation) CountEvent(eventType int, linkURL string, post string, did string, ts time.Time) {
	// Skip event if it is not within the 'previous day' or 'previous week' report.
	if ts.Before(a.bounds.WeekStart) {
		return
	}

	// Check for a duplicate url/event/did combination to prevent spam.
	// i.e. at most, a single user can like, post, and repost a link once.
	// Ensure only one worker is able to update 'fingerprints' at a given moment.
	a.fingerprintsLock.Lock()
	fingerprint := fmt.Sprintf("%s%d%s", linkURL, eventType, did)
	if a.fingerprints.TestAndAddString(fingerprint) {
		a.skipped++
		a.fingerprintsLock.Unlock()
		return
	}
	a.fingerprintsLock.Unlock()

	// Find the shard associated with the given URL
	shard := a.getShard(linkURL)

	shard.lock.Lock()
	if shard.items[linkURL] == nil {
		shard.items[linkURL] = &AggregationItem{}
	}
	shard.items[linkURL].CountEvent(eventType, post, ts, a.bounds)
	shard.lock.Unlock()

	atomic.AddInt64(&a.total, 1)
}

func (a *Aggregation) TopDayLinks(n int) []string {
	// Convert map to slice
	type kv struct {
		URL             string
		AggregationItem *AggregationItem
	}
	var kvs []kv
	for i := range a.shards {
		shard := &a.shards[i]
		for k, v := range shard.items {
			kvs = append(kvs, kv{URL: k, AggregationItem: v})
		}
	}

	// Sort by score
	slices.SortFunc(kvs, func(a, b kv) int {
		scoreA := a.AggregationItem.DayScore()
		scoreB := b.AggregationItem.DayScore()

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

func (a *Aggregation) TopWeekLinks(n int) []string {
	// Convert map to slice
	type kv struct {
		URL             string
		AggregationItem *AggregationItem
	}
	var kvs []kv
	for i := range a.shards {
		shard := &a.shards[i]
		for k, v := range shard.items {
			kvs = append(kvs, kv{URL: k, AggregationItem: v})
		}
	}

	// Sort by score
	slices.SortFunc(kvs, func(a, b kv) int {
		scoreA := a.AggregationItem.WeekScore()
		scoreB := b.AggregationItem.WeekScore()

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

func (a *Aggregation) getShard(url string) *Shard {
	hash := fnv.New32a()
	hash.Write([]byte(url))
	return &a.shards[hash.Sum32()%NumShards]
}
