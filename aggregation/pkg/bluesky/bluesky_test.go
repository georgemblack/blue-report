package bluesky

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/georgemblack/blue-report/pkg/testutil"
)

func TestGetPost(t *testing.T) {
	postURI := "at://did:plc:n26uge5dhwhq7lskqadwx7vx/app.bsky.feed.post/3lhpojagt7s2w"

	// Build mock server
	ms := httptest.NewServer(nil)
	ms.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(testutil.GetTestData("posts.json"))
	})
	defer ms.Close()

	// Build Bluesky service
	bs := New()
	bs.endpoint = ms.URL

	// Execute test
	post, err := bs.GetPost(postURI)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if post.URI != postURI {
		t.Errorf("unexpected post uri: %s", post.URI)
	}
	if post.Author.DisplayName != "George Black" {
		t.Errorf("unexpected author display name: %s", post.Author.DisplayName)
	}
	if post.Author.Handle != "george.black" {
		t.Errorf("unexpected author handle: %s", post.Author.Handle)
	}
	if post.Record.CreatedAt != "2025-02-09T03:23:42.882Z" {
		t.Errorf("unexpected record created at: %s", post.Record.CreatedAt)
	}
}
