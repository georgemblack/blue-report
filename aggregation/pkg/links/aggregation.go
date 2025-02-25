package links

import (
	"fmt"
	"hash/fnv"
	"slices"
	"sync"
	"time"

	"github.com/bits-and-blooms/bloom/v3"
)

const EstimatedTotalEvents = 25000000 // 25 million. Estimate is used to create bloom filter used for duplicate detection.
const DuplicatePrecision = 0.0001     // 0.01% precision for duplicate detection
const NumShards = 1024                // Number of shards to use for parallel processing

type Aggregation struct {
	shards []Shard
	bounds TimeBounds
}

type Shard struct {
	lock         sync.Mutex
	items        map[string]*AggregationItem
	fingerprints *bloom.BloomFilter
	total        int // Number of events processed
	skipped      int // Number of events skipped due to suspected duplicate
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
			fingerprints: bloom.NewWithEstimates(
				EstimatedTotalEvents/NumShards,
				DuplicatePrecision,
			),
			total:   0,
			skipped: 0,
		}
	}

	return Aggregation{
		shards: shards,
		bounds: bounds,
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

func (a *Aggregation) Total() int {
	total := 0

	for i := range a.shards {
		shard := &a.shards[i]
		fmt.Println("shard total", shard.total)
		total += shard.total
	}

	return total
}

func (a *Aggregation) Skipped() int {
	skipped := 0

	for i := range a.shards {
		shard := &a.shards[i]
		fmt.Println("shard skipped", shard.skipped)
		skipped += shard.skipped
	}

	return skipped
}

func (a *Aggregation) CountEvent(eventType int, linkURL string, post string, did string, ts time.Time) {
	// Skip event if it is not within the 'previous day' or 'previous week' report.
	if ts.Before(a.bounds.WeekStart) {
		return
	}

	// Find the shard associated with the given URL
	shard := a.getShard(linkURL)
	shard.lock.Lock()

	fingerprint := fmt.Sprintf("%s%d%s", linkURL, eventType, did)
	if shard.fingerprints.TestAndAddString(fingerprint) {
		shard.skipped++
		shard.lock.Unlock()
		return
	}

	if shard.items[linkURL] == nil {
		shard.items[linkURL] = &AggregationItem{}
	}
	shard.items[linkURL].CountEvent(eventType, post, ts, a.bounds)
	shard.total++
	shard.lock.Unlock()
}

func (a *Aggregation) TopDayLinks(n int) []string {
	kvs := a.toKV()

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
	kvs := a.toKV()

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

type kv struct {
	URL             string
	AggregationItem *AggregationItem
}

func (a *Aggregation) toKV() []kv {
	var kvs []kv

	for i := range a.shards {
		shard := &a.shards[i]
		for k, v := range shard.items {
			kvs = append(kvs, kv{URL: k, AggregationItem: v})
		}
	}

	return kvs
}

func (a *Aggregation) getShard(url string) *Shard {
	hash := fnv.New32a()
	hash.Write([]byte(url))
	return &a.shards[hash.Sum32()%NumShards]
}
