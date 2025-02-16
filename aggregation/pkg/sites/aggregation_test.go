package sites

// func TestAggregationItem(t *testing.T) {
// 	item := AggregationItem{}

// 	item.CountEvent(0, "abc", "did1") // Duplicate
// 	item.CountEvent(0, "abc", "did1") // Duplicate
// 	item.CountEvent(1, "abc", "did1")
// 	item.CountEvent(1, "456", "did2")
// 	item.CountEvent(2, "789", "did3")
// 	item.CountEvent(2, "xyz", "did4")

// 	// In total, there should be 5 interactions, as there is a duplicate event.
// 	if item.Interactions() != 5 {
// 		t.Errorf("expected 5 interactions, got %d", item.Interactions())
// 	}
// }

// func TestAggregation(t *testing.T) {
// 	aggregation := Aggregation{}

// 	aggregation.CountEvent(0, "https://www.example.com/some-page", "did1") // Duplicate
// 	aggregation.CountEvent(0, "https://www.example.com/some-page", "did1") // Duplicate
// 	aggregation.CountEvent(1, "https://example.com/other-page", "did1")
// 	aggregation.CountEvent(1, "https://example.com/other-page", "did2")
// 	aggregation.CountEvent(2, "https://exapmle2.com/other-page", "did3")
// 	aggregation.CountEvent(2, "https://www.exapmle2.com/other-page", "did4")

// 	// There should be two top sites: 'example.com' and 'exapmle2.com'
// 	top := aggregation.TopSites(2)
// 	if len(top) != 2 {
// 		t.Errorf("expected 2 top sites, got %d", len(top))
// 	}
// 	if top[0] != "example.com" {
// 		t.Errorf("expected top site to be example.com, got %s", top[0])
// 	}
// 	if top[1] != "exapmle2.com" {
// 		t.Errorf("expected second top site to be exapmle2.com, got %s", top[1])
// 	}

// 	// Fetch data for 'example.com'
// 	item := aggregation.items["example.com"]

// 	// Total number of interactions should be three (i.e. it ignores one duplicate)
// 	if item.Interactions() != 3 {
// 		t.Errorf("expected three interactions, got %d", item.Interactions())
// 	}

// 	// Top links should be 'other-page' and 'some-page'
// 	topLinks := item.TopLinks(2)
// 	if len(topLinks) != 2 {
// 		t.Errorf("expected two top links, got %d", len(topLinks))
// 	}
// 	if topLinks[0] != "https://example.com/other-page" {
// 		t.Errorf("expected top link to be https://example.com/other-page, got %s", topLinks[0])
// 	}
// 	if topLinks[1] != "https://www.example.com/some-page" {
// 		t.Errorf("expected second top link to be https://www.example.com/some-page, got %s", topLinks[1])
// 	}

// 	// Fetch data for 'exapmle2.com'
// 	item = aggregation.items["exapmle2.com"]

// 	// Total number of interactions should be two
// 	if item.Interactions() != 2 {
// 		t.Errorf("expected two interactions, got %d", item.Interactions())
// 	}
// }
