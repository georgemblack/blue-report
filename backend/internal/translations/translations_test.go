package translations

import "testing"

func check(t *testing.T, tr *Translations, url, want string, wantOK bool) {
	t.Helper()
	got, ok := tr.Final(url)
	if got != want || ok != wantOK {
		t.Fatalf("Final(%q) = %q, %v; want %q, %v", url, got, ok, want, wantOK)
	}
}

func TestNoTranslation(t *testing.T) {
	check(t, New(), "https://example.com/a", "https://example.com/a", true)
}

func TestSingleHop(t *testing.T) {
	tr := New()
	tr.Add("https://nyti.ms/abc", "https://www.nytimes.com/article.html")
	check(t, tr, "https://nyti.ms/abc", "https://www.nytimes.com/article.html", true)
	if tr.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", tr.Len())
	}
}

func TestChain(t *testing.T) {
	tr := New()
	tr.Add("https://a.com", "https://b.com")
	tr.Add("https://b.com", "https://c.com")
	check(t, tr, "https://a.com", "https://c.com", true)
}

func TestChainStopsAtMaxHops(t *testing.T) {
	tr := New()
	urls := []string{"https://0.com", "https://1.com", "https://2.com", "https://3.com", "https://4.com", "https://5.com", "https://6.com"}
	for i := range len(urls) - 1 {
		tr.Add(urls[i], urls[i+1])
	}
	check(t, tr, urls[0], urls[MaxHops], true)
}

func TestLoopEnds(t *testing.T) {
	tr := New()
	tr.Add("https://a.com", "https://b.com")
	tr.Add("https://b.com", "https://a.com")
	if _, ok := tr.Final("https://a.com"); !ok {
		t.Fatal("Final on a loop returned ok=false")
	}
}

func TestDestinationIsCleaned(t *testing.T) {
	tr := New()
	tr.Add("https://nyti.ms/abc", "https://www.nytimes.com/article.html?smid=share#top")
	check(t, tr, "https://nyti.ms/abc", "https://www.nytimes.com/article.html", true)
}

// A destination that cleans back to the source isn't a real redirect.
func TestSelfTranslation(t *testing.T) {
	tr := New()
	tr.Add("https://example.com/a", "https://example.com/a?utm_source=x")
	check(t, tr, "https://example.com/a", "https://example.com/a", true)
}

func TestIgnoredDestination(t *testing.T) {
	tr := New()
	tr.Add("https://short.link/x", "https://media.tenor.com/funny.gif")
	check(t, tr, "https://short.link/x", "", false)
}

func TestIgnoredInMiddleOfChain(t *testing.T) {
	tr := New()
	tr.Add("https://a.com", "http://insecure.com")
	tr.Add("http://insecure.com", "https://c.com")
	check(t, tr, "https://a.com", "", false)
}

func TestIgnoredSource(t *testing.T) {
	check(t, New(), "https://bsky.app/profile/someone", "", false)
	check(t, New(), "", "", false)
}
