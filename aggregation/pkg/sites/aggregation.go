package sites

import (
	"fmt"
	"hash/fnv"
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/bits-and-blooms/bloom/v3"
)

const EstimatedTotalEvents = 110000000 // 110 million. Estimate is used to create bloom filter used for duplicate detection.
const DuplicatePrecision = 0.001       // 0.1% precision for duplicate detection
const NumShards = 512                  // Number of shards to use for parallel processing

type Aggregation struct {
	shards           []Shard
	fingerprints     *bloom.BloomFilter
	fingerprintsLock sync.Mutex
	total            int64 // Number of events processed
	skipped          int64 // Number of events skipped due to suspected duplicate
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

	return Aggregation{
		shards:       shards,
		fingerprints: bloom.NewWithEstimates(EstimatedTotalEvents, DuplicatePrecision),
		total:        0,
		skipped:      0,
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

func (a *Aggregation) CountEvent(eventType int, linkURL string, did string) {
	// Fetch the domain from the URL
	url, err := url.Parse(linkURL)
	if err != nil {
		slog.Debug("failed to parse url when counting event", "url", linkURL)
		return
	}
	host := url.Hostname()

	// Trim 'www.' prefix if it exists
	host = strings.TrimPrefix(host, "www.")

	if host == "" {
		slog.Debug("empty host when parsing url", "url", linkURL)
		return
	}

	// Use bloom filter to detect duplicates.
	a.fingerprintsLock.Lock()
	fingerprint := fmt.Sprintf("%s%d%s", linkURL, eventType, did)
	if a.fingerprints.TestAndAddString(fingerprint) {
		a.skipped++
		a.fingerprintsLock.Unlock()
		return
	}
	a.fingerprintsLock.Unlock()

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

func (a *Aggregation) getShard(url string) *Shard {
	hash := fnv.New32a()
	hash.Write([]byte(url))
	return &a.shards[hash.Sum32()%NumShards]
}
