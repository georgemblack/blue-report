package links

import "testing"

func TestAggregationBasics(t *testing.T) {
	aggregation := NewAggregation()

	aggregation.CountEvent(0, "https://www.example.com/some-page", "abc", "did1")
	item := aggregation.Get("https://www.example.com/some-page")

	if item.Counts.Posts != 1 {
		t.Errorf("expected 1 post, got %d", item.Counts.Posts)
	}
	if aggregation.Total() != 1 {
		t.Errorf("expected 1 total, got %d", aggregation.Total())
	}
	if aggregation.Skipped() != 0 {
		t.Errorf("expected 0 skipped, got %d", aggregation.Skipped())
	}

	top := aggregation.TopLinks(1)
	if len(top) != 1 {
		t.Errorf("expected 1 top link, got %d", len(top))
	}
	if top[0] != "https://www.example.com/some-page" {
		t.Errorf("expected top link to be https://www.example.com/some-page, got %s", top[0])
	}
}

func TestAggregationDuplicateHandling(t *testing.T) {
	aggregation := NewAggregation()

	aggregation.CountEvent(0, "https://www.example.com/some-page", "abc", "did1")
	aggregation.CountEvent(0, "https://www.example.com/some-page", "abc", "did1")
	aggregation.CountEvent(0, "https://www.example2.com/some-page", "xyz", "did2")
	aggregation.CountEvent(0, "https://www.example2.com/some-page", "123", "did3")

	if aggregation.Total() != 3 {
		t.Errorf("expected 2 total, got %d", aggregation.Total())
	}
	if aggregation.Skipped() != 1 {
		t.Errorf("expected 1 skipped, got %d", aggregation.Skipped())
	}

	top := aggregation.TopLinks(2)
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
