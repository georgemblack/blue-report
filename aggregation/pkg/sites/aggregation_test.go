package sites

import "testing"

func TestAggregationItem(t *testing.T) {
	item := AggregationItem{}

	item.CountEvent(0, "abc", "did1") // Duplicate
	item.CountEvent(0, "abc", "did1") // Duplicate
	item.CountEvent(1, "abc", "did1")
	item.CountEvent(1, "456", "did2")
	item.CountEvent(2, "789", "did3")
	item.CountEvent(2, "xyz", "did4")

	// In total, there should be 5 interactions, as there is a duplicate event.
	if item.Interactions() != 5 {
		t.Errorf("expected 5 interactions, got %d", item.Interactions())
	}
}
