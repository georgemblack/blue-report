package links

import (
	"slices"
	"time"
)

// Keep track of two separate counts, for the 'previous day' and 'previous week' report.
type AggregationItem struct {
	WeekCount Counts
	DayCount  Counts
	HourCount Counts
	Posts     map[string]int
}

type Counts struct {
	Posts   int
	Reposts int
	Likes   int
}

func (a *AggregationItem) HourScore() int {
	return (a.HourCount.Posts * 10) + (a.HourCount.Reposts * 10) + a.HourCount.Likes
}

func (a *AggregationItem) DayScore() int {
	return (a.DayCount.Posts * 10) + (a.DayCount.Reposts * 10) + a.DayCount.Likes
}

func (a *AggregationItem) WeekScore() int {
	return (a.WeekCount.Posts * 10) + (a.WeekCount.Reposts * 10) + a.WeekCount.Likes
}

func (a *AggregationItem) CountEvent(eventType int, post string, ts time.Time, bnds TimeBounds) {
	// Check if the event should be counted in the 'previous hour' report.
	if ts.After(bnds.HourStart) {
		if eventType == 0 {
			a.HourCount.Posts++
		}
		if eventType == 1 {
			a.HourCount.Reposts++
		}
		if eventType == 2 {
			a.HourCount.Likes++
		}
	}

	// Check if the event should be counted in the 'previous day' report.
	if ts.After(bnds.DayStart) {
		if eventType == 0 {
			a.DayCount.Posts++
		}
		if eventType == 1 {
			a.DayCount.Reposts++
		}
		if eventType == 2 {
			a.DayCount.Likes++
		}
	}

	// Assume all events are within the 'previous week' report.
	// This is checked in the caller.
	if eventType == 0 {
		a.WeekCount.Posts++
	}
	if eventType == 1 {
		a.WeekCount.Reposts++
	}
	if eventType == 2 {
		a.WeekCount.Likes++
	}

	// Add AT URI of post to map, and increment number of interactions
	if a.Posts == nil {
		a.Posts = make(map[string]int)
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
