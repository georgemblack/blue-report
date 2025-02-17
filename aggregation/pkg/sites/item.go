package sites

import "slices"

type AggregationItem struct {
	links  map[string]Counts // Track each URL and the associated posts/resposts/likes
	counts Counts            // Track total posts/resposts/likes for site
}

type Counts struct {
	Posts   int
	Reposts int
	Likes   int
}

func (c Counts) Total() int {
	return c.Posts + c.Reposts + c.Likes
}

func (a *AggregationItem) CountEvent(eventType int, linkURL string, did string) {
	if a.links == nil {
		a.links = make(map[string]Counts)
	}

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
