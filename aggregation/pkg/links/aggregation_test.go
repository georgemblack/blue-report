package links

import (
	"testing"
	"time"
)

func TestAggregationBasics(t *testing.T) {
	now := time.Now().UTC()
	bounds := TimeBounds{
		DayStart:  now.Add(-24 * time.Hour),
		WeekStart: now.Add(-24 * 7 * time.Hour),
	}
	aggregation := NewAggregation(bounds)

	ts := now.Add(-1 * time.Minute)
	aggregation.CountEvent(0, "https://www.example.com/some-page", "abc", "did1", ts)
	item := aggregation.Get("https://www.example.com/some-page")

	if item.DayCount.Posts != 1 {
		t.Errorf("expected 1 post, got %d", item.DayCount.Posts)
	}
	if aggregation.Total() != 1 {
		t.Errorf("expected 1 total, got %d", aggregation.Total())
	}
	if aggregation.Skipped() != 0 {
		t.Errorf("expected 0 skipped, got %d", aggregation.Skipped())
	}

	top := aggregation.TopDayLinks(1)
	if len(top) != 1 {
		t.Errorf("expected 1 top link, got %d", len(top))
	}
	if top[0] != "https://www.example.com/some-page" {
		t.Errorf("expected top link to be https://www.example.com/some-page, got %s", top[0])
	}
}

func TestAggregationDuplicateHandling(t *testing.T) {
	now := time.Now().UTC()
	bounds := TimeBounds{
		DayStart:  now.Add(-24 * time.Hour),
		WeekStart: now.Add(-24 * 7 * time.Hour),
	}
	aggregation := NewAggregation(bounds)

	ts := now.Add(-1 * time.Minute)
	aggregation.CountEvent(0, "https://www.example.com/some-page", "abc", "did1", ts)
	aggregation.CountEvent(0, "https://www.example.com/some-page", "abc", "did1", ts)
	aggregation.CountEvent(0, "https://www.example2.com/some-page", "xyz", "did2", ts)
	aggregation.CountEvent(0, "https://www.example2.com/some-page", "123", "did3", ts)

	if aggregation.Total() != 3 {
		t.Errorf("expected 3 total, got %d", aggregation.Total())
	}
	if aggregation.Skipped() != 1 {
		t.Errorf("expected 1 skipped, got %d", aggregation.Skipped())
	}

	top := aggregation.TopDayLinks(2)
	if len(top) != 2 {
		t.Errorf("expected 2 top links, got %d", len(top))
	}
	if top[0] != "https://www.example2.com/some-page" {
		t.Errorf("expected top link to be https://www.example2.com/some-page, got %s", top[0])
	}
	if top[1] != "https://www.example.com/some-page" {
		t.Errorf("expected second top link to be https://www.example.com/some-page, got %s", top[1])
	}
}

func TestAggregationTimeHandling(t *testing.T) {
	now := time.Now().UTC()
	bounds := TimeBounds{
		DayStart:  now.Add(-24 * time.Hour),
		WeekStart: now.Add(-24 * 7 * time.Hour),
	}
	aggregation := NewAggregation(bounds)

	inDayBounds := now.Add(-100 * time.Minute)
	inWeekBounds := now.Add(-24 * 2 * time.Hour)
	outOfBounds := now.Add(-24 * 8 * time.Hour)

	aggregation.CountEvent(0, "https://www.example2.com", "abc", "did1", inWeekBounds)
	aggregation.CountEvent(0, "https://www.example2.com", "abc", "did2", inWeekBounds)
	aggregation.CountEvent(0, "https://www.example2.com", "xyz", "did3", inWeekBounds)
	aggregation.CountEvent(0, "https://www.example.com/some-page", "abc", "did1", inDayBounds)
	aggregation.CountEvent(0, "https://www.example.com/some-page", "xyz", "did2", inDayBounds)
	aggregation.CountEvent(0, "https://www.example.com/some-page", "abc", "did3", outOfBounds)
	aggregation.CountEvent(0, "https://www.example.com/some-page", "abc", "did4", outOfBounds)

	if aggregation.Total() != 5 {
		t.Errorf("expected 5 total, got %d", aggregation.Total())
	}
	if aggregation.Skipped() != 0 {
		t.Errorf("expected 0 skipped, got %d", aggregation.Skipped())
	}

	top := aggregation.TopDayLinks(1)
	if len(top) != 1 {
		t.Errorf("expected 1 top links, got %d", len(top))
	}
	if top[0] != "https://www.example.com/some-page" {
		t.Errorf("expected top link to be https://www.example.com/some-page, got %s", top[0])
	}

	top = aggregation.TopWeekLinks(2)
	if len(top) != 2 {
		t.Errorf("expected 2 top links, got %d", len(top))
	}
	if top[0] != "https://www.example2.com" {
		t.Errorf("expected top link to be https://www.example2.com, got %s", top[0])
	}
	if top[1] != "https://www.example.com/some-page" {
		t.Errorf("expected second top link to be https://www.example.com/some-page, got %s", top[1])
	}

	item := aggregation.Get("https://www.example2.com")
	top = item.TopPosts()
	if len(top) != 2 {
		t.Errorf("expected 2 top posts, got %d", len(top))
	}
	if top[0] != "abc" {
		t.Errorf("expected top post to be abc, got %s", top[0])
	}
	if top[1] != "xyz" {
		t.Errorf("expected second top post to be xyz, got %s", top[1])
	}
}
