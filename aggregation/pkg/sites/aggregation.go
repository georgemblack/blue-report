package sites

import (
	"fmt"
	"slices"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/georgemblack/blue-report/pkg/util"
)

type Aggregation map[string]AggregationItem

type AggregationItem struct {
	links        map[string]Counts  // Track each URL and the associated posts/resposts/likes
	counts       Counts             // Track total posts/resposts/likes for site
	fingerprints mapset.Set[string] // Track unique DID, URL, and event type combinations
}

type Counts struct {
	Posts   int
	Reposts int
	Likes   int
}

func (c Counts) Total() int {
	return c.Posts + c.Reposts + c.Likes
}

func (a *Aggregation) CountEvent(eventType int, url string, did string) {
	if *a == nil {
		*a = make(Aggregation)
	}

	if _, ok := (*a)[url]; !ok {
		(*a)[url] = AggregationItem{}
	}

	item := (*a)[url]
	item.CountEvent(eventType, url, did)
	(*a)[url] = item
}

func (a *Aggregation) TopSites(n int) []string {
	// Convert map to slice
	type kv struct {
		Domain          string
		AggregationItem AggregationItem
	}

	var kvs []kv
	for k, v := range *a {
		kvs = append(kvs, kv{Domain: k, AggregationItem: v})
	}

	// Sort by total interactions
	slices.SortFunc(kvs, func(a, b kv) int {
		scoreA := a.AggregationItem.Interactions()
		scoreB := b.AggregationItem.Interactions()

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

func (a *AggregationItem) CountEvent(eventType int, url string, did string) {
	if a.fingerprints == nil {
		a.fingerprints = mapset.NewSet[string]()
	}
	if a.links == nil {
		a.links = make(map[string]Counts)
	}
	if _, ok := a.links[url]; !ok {
		a.links[url] = Counts{}
	}

	// Avoid duplicate post/repost/like from same user/url combo
	fingerprint := fmt.Sprintf("%d%s%s", eventType, util.Hash(url), did)
	if a.fingerprints.Contains(fingerprint) {
		return
	}
	a.fingerprints.Add(fingerprint)

	// Increment:
	//	- The count for the given URL
	// 	- The count for the site as a whole
	item := a.links[url]
	if eventType == 0 {
		item.Posts++
		a.counts.Posts++
	}
	if eventType == 1 {
		item.Reposts++
		a.counts.Reposts++
	}
	if eventType == 2 {
		item.Likes++
		a.counts.Likes++
	}
	a.links[url] = item
}

func (a *AggregationItem) Interactions() int {
	return a.counts.Total()
}

func (a *AggregationItem) TopLinks(n int) []string {
	// Convert map to slice
	type kv struct {
		URL    string
		Counts Counts
	}

	var kvs []kv
	for k, v := range a.links {
		kvs = append(kvs, kv{URL: k, Counts: v})
	}

	// Sort by interactions
	slices.SortFunc(kvs, func(a, b kv) int {
		scoreA := a.Counts.Total()
		scoreB := b.Counts.Total()

		if scoreA > scoreB {
			return -1
		}
		if scoreA < scoreB {
			return 1
		}
		return 0
	})

	// Find top n items
	links := make([]string, 0, n)
	for i := range kvs {
		if len(links) >= n {
			break
		}
		links = append(links, kvs[i].URL)
	}

	return links
}
