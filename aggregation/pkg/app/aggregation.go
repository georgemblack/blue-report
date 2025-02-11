package app

import "slices"

// Aggregation is a map of 'URL' -> 'URLAggregation'.
type Aggregation map[string]URLAggregation

// URLAggregation represents all aggregated data for a URL. Specifically:
//   - The post/repost/like count
//   - The AT URI of each post referencing the URL, and its number of interactions
type URLAggregation struct {
	Counts struct {
		Posts   int
		Reposts int
		Likes   int
	}
	Posts map[string]Interactions
}

type Interactions struct {
	Total int
}

func (a *URLAggregation) IncrementPostCount() {
	a.Counts.Posts++
}

func (a *URLAggregation) IncrementRepostCount() {
	a.Counts.Reposts++
}

func (a *URLAggregation) IncrementLikeCount() {
	a.Counts.Likes++
}

func (a *URLAggregation) CountPost(atURI string) {
	if a.Posts == nil {
		a.Posts = make(map[string]Interactions)
	}

	if _, ok := a.Posts[atURI]; !ok {
		a.Posts[atURI] = Interactions{Total: 0}
	}

	interactions := a.Posts[atURI]
	interactions.Total++
	a.Posts[atURI] = interactions
}

// Score determins a URL's rank on the final report.
// A post is worth 10 points, a repost 10 points, and a like 1 point.
func (a *URLAggregation) Score() int {
	return (a.Counts.Posts * 10) + (a.Counts.Reposts * 10) + a.Counts.Likes
}

// TopPosts returns the AT URIs of the top ten posts referencing the URL, based on the number of interactions.
func (a *URLAggregation) TopPosts() []string {
	// Convert map to slice
	type kv struct {
		Post         string
		Interactions Interactions
	}
	var kvs []kv
	for k, v := range a.Posts {
		kvs = append(kvs, kv{k, v})
	}

	// Sort by interactions
	slices.SortFunc(kvs, func(a, b kv) int {
		return b.Interactions.Total - a.Interactions.Total
	})

	// Return the top ten
	top := make([]string, 0, 20)
	for i := range kvs {
		if len(top) >= 20 {
			break
		}
		top = append(top, kvs[i].Post)
	}

	return top
}
