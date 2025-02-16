package links

// func TestAggregationCounting(t *testing.T) {
// 	agg := URLAggregation{}
// 	for i := 0; i < 5; i++ {
// 		agg.IncrementPostCount()
// 	}
// 	for i := 0; i < 3; i++ {
// 		agg.IncrementRepostCount()
// 	}
// 	for i := 0; i < 7; i++ {
// 		agg.IncrementLikeCount()
// 	}

// 	if agg.Counts.Posts != 5 {
// 		t.Errorf("unexpected post count: %d", agg.Counts.Posts)
// 	}
// 	if agg.Counts.Reposts != 3 {
// 		t.Errorf("unexpected repost count: %d", agg.Counts.Reposts)
// 	}
// 	if agg.Counts.Likes != 7 {
// 		t.Errorf("unexpected like count: %d", agg.Counts.Likes)
// 	}
// }

// func TestAggregationTopPosts(t *testing.T) {
// 	agg := URLAggregation{}
// 	for i := 0; i < 5; i++ {
// 		agg.CountPost("456")
// 	}
// 	for i := 0; i < 10; i++ {
// 		agg.CountPost("123")
// 	}
// 	for i := 0; i < 20; i++ {
// 		agg.CountPost("yz")
// 	}
// 	for i := 0; i < 30; i++ {
// 		agg.CountPost("vwx")
// 	}
// 	for i := 0; i < 40; i++ {
// 		agg.CountPost("stu")
// 	}
// 	for i := 0; i < 50; i++ {
// 		agg.CountPost("pqr")
// 	}
// 	for i := 0; i < 60; i++ {
// 		agg.CountPost("mno")
// 	}
// 	for i := 0; i < 70; i++ {
// 		agg.CountPost("jkl")
// 	}
// 	for i := 0; i < 80; i++ {
// 		agg.CountPost("ghi")
// 	}
// 	for i := 0; i < 90; i++ {
// 		agg.CountPost("def")
// 	}
// 	for i := 0; i < 100; i++ {
// 		agg.CountPost("abc")
// 	}

// 	expected := []string{"abc", "def", "ghi", "jkl", "mno", "pqr", "stu", "vwx", "yz", "123"}
// 	result := agg.TopPosts()

// 	for i, post := range result {
// 		if post != expected[i] {
// 			t.Errorf("unexpected post at index %d: %s", i, post)
// 		}
// 	}
// }

// func TestAggregationTopPostsLessThanTen(t *testing.T) {
// 	agg := URLAggregation{}
// 	for i := 0; i < 5; i++ {
// 		agg.CountPost("456")
// 	}
// 	for i := 0; i < 10; i++ {
// 		agg.CountPost("123")
// 	}
// 	for i := 0; i < 20; i++ {
// 		agg.CountPost("yz")
// 	}

// 	expected := []string{"yz", "123", "456"}
// 	result := agg.TopPosts()

// 	if len(result) != 3 {
// 		t.Errorf("unexpected post count: %d", len(result))
// 	}
// 	for i, post := range result {
// 		if post != expected[i] {
// 			t.Errorf("unexpected post at index %d: %s", i, post)
// 		}
// 	}
// }
