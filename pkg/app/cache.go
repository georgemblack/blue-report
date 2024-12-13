package app

import "time"

// EventCache represents an internal, in-memory cache of event records.
// This is not to be confused with Valkey, which is our external data store.
type EventCache struct {
	Cache map[string]CacheRecord
}

type CacheRecord struct {
	Record EventRecord
	Expiry time.Time
}

func (r CacheRecord) Expired() bool {
	return time.Now().After(r.Expiry)
}

type CacheDump struct {
	Items []CacheDumpItem `json:"items"`
}

type CacheDumpItem struct {
	Key   string      `json:"key"`
	Value CacheRecord `json:"value"`
}

func NewEventCache() EventCache {
	return EventCache{Cache: map[string]CacheRecord{}}
}

func (c *EventCache) Populate(dump CacheDump) {
	for _, item := range dump.Items {
		c.Cache[item.Key] = item.Value
	}
}

func (c *EventCache) Dump() CacheDump {
	items := make([]CacheDumpItem, 0, c.Len())
	for k, v := range c.Cache {
		items = append(items, CacheDumpItem{Key: k, Value: v})
	}
	return CacheDump{Items: items}
}

func (c *EventCache) Add(key string, value CacheRecord) {
	c.Cache[key] = value
}

func (c *EventCache) Get(key string) (CacheRecord, bool) {
	v, ok := c.Cache[key]
	return v, ok
}

func (c *EventCache) Delete(key string) {
	delete(c.Cache, key)
}

func (c *EventCache) Len() int {
	return len(c.Cache)
}
