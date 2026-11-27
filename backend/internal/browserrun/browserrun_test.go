package browserrun

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestScrape(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "0") // treated as 1 second
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		if r.URL.Path != "/scrape" || r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("request to %s with %q", r.URL.Path, r.Header.Get("Authorization"))
		}
		var body struct {
			URL      string `json:"url"`
			Elements []struct {
				Selector string `json:"selector"`
			} `json:"elements"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if body.URL != "https://example.com/a" || len(body.Elements) != 2 {
			t.Errorf("body = %+v", body)
		}
		w.Header().Set("X-Browser-Ms-Used", "1500.25")
		w.Write([]byte(`{"success": true, "result": [
			{"selector": "title", "results": [{"text": "A page", "attributes": []}]},
			{"selector": "meta[property=\"og:image\"]", "results": [
				{"text": "", "attributes": [{"name": "property", "value": "og:image"}, {"name": "content", "value": "https://example.com/a.jpg"}]}
			]}
		]}`))
	}))
	defer server.Close()

	c := New("account", "token")
	c.baseURL = server.URL
	res, used, err := c.Scrape(context.Background(), "https://example.com/a", []string{"title", `meta[property="og:image"]`})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text("title") != "A page" || res.Attr(`meta[property="og:image"]`, "content") != "https://example.com/a.jpg" {
		t.Fatalf("result = %+v", res)
	}
	if res.Text("missing") != "" || res.Attr("missing", "content") != "" {
		t.Fatal("missing selector returned something")
	}
	if used != 1500250*time.Microsecond || calls != 2 {
		t.Fatalf("used = %v, calls = %d", used, calls)
	}
}

func TestScrapeFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer server.Close()
	c := New("account", "token")
	c.baseURL = server.URL
	if _, _, err := c.Scrape(context.Background(), "https://example.com/a", []string{"title"}); err == nil {
		t.Fatal("no error")
	}
}
