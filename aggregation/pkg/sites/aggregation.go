package sites

import (
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/georgemblack/blue-report/pkg/util"
)

type Aggregation map[string]AggregationItem

type AggregationItem struct {
	links        map[string]Counts  // Track each URL and the associated posts/resposts/likes
	counts       Counts             // Track total posts/resposts/likes for site
	fingerprints mapset.Set[string] // Track unique DID, URL, and event type combinations
	skipped      int                // Track the number of skipped events based on fingerprint
}

type Counts struct {
	Posts   int
	Reposts int
	Likes   int
}

func (c Counts) Total() int {
	return c.Posts + c.Reposts + c.Likes
}

// Count an event for a given URL.
// URLs should be filtered and normalized before being a part of the aggregation.
func (a *Aggregation) CountEvent(eventType int, linkURL string, did string) {
	// Create the map if it doesn't exist
	if *a == nil {
		*a = make(Aggregation)
	}

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

	// Create aggregation item for host if it doesn't eixst
	if _, ok := (*a)[host]; !ok {
		(*a)[host] = AggregationItem{}
	}

	item := (*a)[host]
	item.CountEvent(eventType, linkURL, did)
	(*a)[host] = item
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

func (a *Aggregation) Skipped() int {
	total := 0

	// For each item in map
	for _, item := range *a {
		total += item.Skipped()
	}

	return total
}

func (a *AggregationItem) CountEvent(eventType int, linkURL string, did string) {
	if a.fingerprints == nil {
		a.fingerprints = mapset.NewSet[string]()
	}
	if a.links == nil {
		a.links = make(map[string]Counts)
	}
	if _, ok := a.links[linkURL]; !ok {
		a.links[linkURL] = Counts{}
	}

	// Avoid duplicate post/repost/like from same user/url combo
	fingerprint := fmt.Sprintf("%d%s%s", eventType, util.Hash(linkURL), did)
	if a.fingerprints.Contains(fingerprint) {
		a.skipped++
		return
	}
	a.fingerprints.Add(fingerprint)

	// Increment:
	//	- The count for the given URL
	// 	- The count for the site as a whole
	item := a.links[linkURL]
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
	a.links[linkURL] = item
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

func (a *AggregationItem) Interactions() int {
	return a.counts.Total()
}

func (a *AggregationItem) Skipped() int {
	return a.skipped
}
