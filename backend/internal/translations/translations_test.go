package translations

import "testing"

func TestGet(t *testing.T) {
	tr := New()
	if got := tr.Get("https://example.com/a"); got != "https://example.com/a" {
		t.Fatalf("unknown link: Get = %q", got)
	}
	tr.Add("https://nyti.ms/abc", "https://www.nytimes.com/article.html")
	if got := tr.Get("https://nyti.ms/abc"); got != "https://www.nytimes.com/article.html" {
		t.Fatalf("Get = %q", got)
	}
	if tr.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", tr.Len())
	}
}

// Only one lookup is made: chains aren't followed.
func TestNoChains(t *testing.T) {
	tr := New()
	tr.Add("https://a.com", "https://b.com")
	tr.Add("https://b.com", "https://c.com")
	if got := tr.Get("https://a.com"); got != "https://b.com" {
		t.Fatalf("Get = %q", got)
	}
}
