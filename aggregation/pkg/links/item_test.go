package links

import (
	"testing"
	"time"
)

func TestAggregationItem(t *testing.T) {
	now := time.Now().UTC()
	bounds := TimeBounds{
		DayStart:  now.Add(-24 * time.Hour),
		WeekStart: now.Add(-24 * 7 * time.Hour),
	}
	item := AggregationItem{}

	ts := now.Add(-1 * time.Minute)
	item.CountEvent(0, "abc", ts, bounds)
	item.CountEvent(1, "abc", ts, bounds)
	item.CountEvent(1, "abc", ts, bounds)
	item.CountEvent(2, "xyz", ts, bounds)
	item.CountEvent(2, "xyz", ts, bounds)

	if item.DayScore() != 32 {
		t.Errorf("unexpected score: %d", item.DayScore())
	}

	top := item.TopPosts()
	if len(top) != 2 {
		t.Errorf("unexpected top post count: %d", len(top))
	}
	if top[0] != "abc" {
		t.Errorf("unexpected top post: %s", top[0])
	}
	if top[1] != "xyz" {
		t.Errorf("unexpected top post: %s", top[1])
	}
}
