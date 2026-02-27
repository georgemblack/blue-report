package links

import (
	"hash/fnv"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bits-and-blooms/bloom/v3"
)

const EstimatedTotalEvents = 25000000 // 25 million. Estimate is used to create bloom filter used for duplicate detection.
const DuplicatePrecision = 0.001      // 0.1% precision for duplicate detection
const NumShards = 1024                // Number of shards to use for parallel processing
const NumBloomShards = 64             // Number of shards to spread bloom filter contention

type Aggregation struct {
	shards      []Shard
	bloomShards []BloomShard
	bounds      TimeBounds
	total       int64 // Number of events processed
	skipped     int64 // Number of events skipped due to suspected duplicate
}

type BloomShard struct {
	lock   sync.Mutex
	filter *bloom.BloomFilter
}

type Shard struct {
	lock  sync.Mutex
	items map[string]*AggregationItem
}

type TimeBounds struct {
	HourStart time.Time // Start of the 'past hour' report
	DayStart  time.Time // Start of the 'past day' report
	WeekStart time.Time // Start of the 'past week' report
}

func NewAggregation(bounds TimeBounds) Aggregation {
	shards := make([]Shard, NumShards)
	for i := range shards {
		shards[i] = Shard{
			lock:  sync.Mutex{},
			items: make(map[string]*AggregationItem),
		}
	}

	bloomShards := make([]BloomShard, NumBloomShards)
	for i := range bloomShards {
		bloomShards[i] = BloomShard{
			filter: bloom.NewWithEstimates(EstimatedTotalEvents/NumBloomShards, DuplicatePrecision),
		}
	}

	return Aggregation{
		shards:      shards,
		bloomShards: bloomShards,
		bounds:      bounds,
		total:       0,
		skipped:     0,
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
	// Skip event if it is not within a time boundary for any report
	if ts.Before(a.bounds.WeekStart) {
		return
	}

	// Check for a duplicate url/event/did combination to prevent spam.
	// i.e. at most, a single user can like, post, and repost a link once.
	// String concatenation is faster than fmt.Sprintf for this use case.
	fingerprint := linkURL + strconv.Itoa(eventType) + did
	bloomShard := a.getBloomShard(fingerprint)

	bloomShard.lock.Lock()
	if bloomShard.filter.TestAndAddString(fingerprint) {
		atomic.AddInt64(&a.skipped, 1)
		bloomShard.lock.Unlock()
		return
	}
	bloomShard.lock.Unlock()

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

func (a *Aggregation) TopHourLinks(n int) []string {
	kvs := a.toKV()

	// Sort by score
	slices.SortFunc(kvs, func(a, b kv) int {
		scoreA := a.AggregationItem.HourScore()
		scoreB := b.AggregationItem.HourScore()

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

// Convert map to slice that can be sorted
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

func (a *Aggregation) getShard(key string) *Shard {
	return &a.shards[fnv32(key)%NumShards]
}

func (a *Aggregation) getBloomShard(key string) *BloomShard {
	return &a.bloomShards[fnv32(key)%NumBloomShards]
}

func fnv32(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
