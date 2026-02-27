package sites

import (
	"hash/fnv"
	"log/slog"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/bits-and-blooms/bloom/v3"
)

const EstimatedTotalEvents = 110000000 // 110 million. Estimate is used to create bloom filter used for duplicate detection.
const DuplicatePrecision = 0.001       // 0.1% precision for duplicate detection
const NumShards = 512                  // Number of shards to use for parallel processing
const NumBloomShards = 64              // Number of shards to spread bloom filter contention

type Aggregation struct {
	shards      []Shard
	bloomShards []BloomShard
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

func NewAggregation() Aggregation {
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
		total:       0,
		skipped:     0,
	}
}

func (a *Aggregation) Get(host string) AggregationItem {
	shard := a.getShard(host)
	result := shard.items[host]
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

func (a *Aggregation) CountEvent(eventType int, linkURL string, host string, did string) {
	if host == "" {
		slog.Debug("empty host when counting event", "url", linkURL)
		return
	}

	// Use sharded bloom filter to detect duplicates.
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

	// Find the shard associated with the given host
	shard := a.getShard(host)

	shard.lock.Lock()
	if shard.items[host] == nil {
		shard.items[host] = &AggregationItem{}
	}
	shard.items[host].CountEvent(eventType, linkURL, did)
	shard.lock.Unlock()

	atomic.AddInt64(&a.total, 1)
}

func (a *Aggregation) TopSites(n int) []string {
	// Convert map to slice
	type kv struct {
		Domain          string
		AggregationItem *AggregationItem
	}

	var kvs []kv
	for i := range a.shards {
		shard := &a.shards[i]
		for k, v := range shard.items {
			kvs = append(kvs, kv{Domain: k, AggregationItem: v})
		}
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
