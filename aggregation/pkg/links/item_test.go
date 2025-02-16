package links

import "testing"

func TestAggregationItem(t *testing.T) {
	item := AggregationItem{}

	item.CountEvent(0, "abc")
	item.CountEvent(1, "abc")
	item.CountEvent(1, "abc")
	item.CountEvent(2, "xyz")
	item.CountEvent(2, "xyz")

	if item.Score() != 32 {
		t.Errorf("unexpected score: %d", item.Score())
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
