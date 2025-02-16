package links

import "slices"

type AggregationItem struct {
	Counts Counts
	Posts  map[string]int
}

type Counts struct {
	Posts   int
	Reposts int
	Likes   int
}

// Score determins a URL's rank on the final report.
// A post is worth 10 points, a repost 10 points, and a like 1 point.
func (a *AggregationItem) Score() int {
	return (a.Counts.Posts * 10) + (a.Counts.Reposts * 10) + a.Counts.Likes
}

func (a *AggregationItem) CountEvent(eventType int, post string) {
	// Increment counts based on event type
	if eventType == 0 {
		a.Counts.Posts++
	}
	if eventType == 1 {
		a.Counts.Reposts++
	}
	if eventType == 2 {
		a.Counts.Likes++
	}

	// Add AT URI of post to map, and increment number of interactions
	if a.Posts == nil {
		a.Posts = make(map[string]int)
	}
	if _, ok := a.Posts[post]; !ok {
		a.Posts[post] = 0
	}
	a.Posts[post]++
}

// TopPosts returns the AT URIs of the top ten posts referencing the URL, based on the number of interactions.
func (a *AggregationItem) TopPosts() []string {
	// Convert map to slice
	type kv struct {
		Post         string
		Interactions int
	}
	var kvs []kv
	for k, v := range a.Posts {
		kvs = append(kvs, kv{k, v})
	}

	// Sort by interactions
	slices.SortFunc(kvs, func(a, b kv) int {
		return b.Interactions - a.Interactions
	})

	// Return the top 20
	top := make([]string, 0, 20)
	for i := range kvs {
		if len(top) >= 20 {
			break
		}
		top = append(top, kvs[i].Post)
	}

	return top
}
