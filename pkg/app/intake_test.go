package app_test

import (
	"testing"

	"github.com/georgemblack/blue-report/pkg/app"
	"github.com/georgemblack/blue-report/pkg/app/util"
	"github.com/georgemblack/blue-report/pkg/cache"
	"github.com/georgemblack/blue-report/pkg/storage"
	"github.com/georgemblack/blue-report/pkg/testutil"
	"go.uber.org/mock/gomock"
)

func TestHandlePostNoURL(t *testing.T) {
	event, err := testutil.GetStreamEvent("post-no-url.json")
	if err != nil {
		t.Fatal(err)
	}

	// The cache should not be called – the event does not contain a URL
	mockCache := testutil.NewMockCache(gomock.NewController(t))
	mockCache.EXPECT().ReadPost(gomock.Any()).Times(0)
	mockCache.EXPECT().SavePost(gomock.Any(), gomock.Any()).Times(0)

	_, _, skip, err := app.HandlePost(mockCache, event)

	if !skip {
		t.Error("expected event to be skipped")
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestHandlePostWithEmbed(t *testing.T) {
	event, err := testutil.GetStreamEvent("post-embed-only.json")
	if err != nil {
		t.Fatal(err)
	}

	expectedPost := cache.PostRecord{
		URL: "https://tylervigen.com/the-mystery-of-the-bloomfield-bridge",
	}
	expectedURL := cache.URLRecord{
		Title:    "The Mystery of the Bloomfield Bridge",
		ImageURL: "https://cdn.bsky.app/img/feed_thumbnail/plain/did:plc:ruzlll5u7u7pfxybmppqyxbx/bafkreiasj4bgohn7rx2mhf3i4r7tdr43kuyyks6cxsgi5zuttq4274ibny",
		Totals: cache.Totals{
			Posts: 1,
		},
	}
	hashedCID := util.Hash("bafyreiehzp2ehowobuutnjsednkq24iisx2mzpdc27yuy4xztspcqid3ni")
	hashedURL := util.Hash(expectedPost.URL)

	mockCache := testutil.NewMockCache(gomock.NewController(t))
	mockCache.EXPECT().SavePost(hashedCID, expectedPost)                 // Save the post
	mockCache.EXPECT().ReadURL(hashedURL).Return(cache.URLRecord{}, nil) // Check for existing URL record (it doesn't exist)

	_, url, skip, err := app.HandlePost(mockCache, event)

	if skip {
		t.Error("unexpected event skip")
	}
	if err != nil {
		t.Fatal(err)
	}
	if url.Title != expectedURL.Title {
		t.Errorf("unexpected title: %s", url.Title)
	}
	if url.ImageURL != expectedURL.ImageURL {
		t.Errorf("unexpected image url: %s", url.ImageURL)
	}
	if url.Totals.Posts != expectedURL.Totals.Posts {
		t.Errorf("unexpected post count: %d", url.Totals.Posts)
	}
}

// Save a post with a URL that has already been cached. The cached URL is missing data, so it should be updated.
func TestHandlePostWithPartiallySavedURL(t *testing.T) {
	event, err := testutil.GetStreamEvent("post-embed-only.json")
	if err != nil {
		t.Fatal(err)
	}

	expectedPost := cache.PostRecord{
		URL: "https://tylervigen.com/the-mystery-of-the-bloomfield-bridge",
	}
	existingURL := cache.URLRecord{
		Title: "The Mystery of the Bloomfield Bridge",
		Totals: cache.Totals{
			Posts: 1,
		},
	}
	expectedURL := cache.URLRecord{
		Title:    "The Mystery of the Bloomfield Bridge",
		ImageURL: "https://cdn.bsky.app/img/feed_thumbnail/plain/did:plc:ruzlll5u7u7pfxybmppqyxbx/bafkreiasj4bgohn7rx2mhf3i4r7tdr43kuyyks6cxsgi5zuttq4274ibny",
		Totals: cache.Totals{
			Posts: 2,
		},
	}
	hashedCID := util.Hash("bafyreiehzp2ehowobuutnjsednkq24iisx2mzpdc27yuy4xztspcqid3ni")
	hashedURL := util.Hash(expectedPost.URL)

	mockCache := testutil.NewMockCache(gomock.NewController(t))
	mockCache.EXPECT().SavePost(hashedCID, expectedPost)           // Save the post
	mockCache.EXPECT().ReadURL(hashedURL).Return(existingURL, nil) // Check for existing URL record (it exists, with missing data)

	_, url, skip, err := app.HandlePost(mockCache, event)

	if skip {
		t.Error("unexpected event skip")
	}
	if err != nil {
		t.Fatal(err)
	}
	if url.Title != expectedURL.Title {
		t.Errorf("unexpected title: %s", url.Title)
	}
	if url.ImageURL != expectedURL.ImageURL {
		t.Errorf("unexpected image url: %s", url.ImageURL)
	}
	if url.Totals.Posts != expectedURL.Totals.Posts {
		t.Errorf("unexpected post count: %d", url.Totals.Posts)
	}
}

// Test handling a quote post
func TestHandleQuotePost(t *testing.T) {
	event, err := testutil.GetStreamEvent("post-quote.json")
	if err != nil {
		t.Fatal(err)
	}

	existingURL := cache.URLRecord{
		Totals: cache.Totals{
			Posts: 1,
		},
	}
	expectedRecord := storage.EventRecord{
		Type: 0,
		URL:  "https://tylervigen.com/the-mystery-of-the-bloomfield-bridge",
		DID:  "did:plc:ruzlll5u7u7pfxybmppqyxbx",
	}

	hashedCID := util.Hash("bafyreihhlj7nktvq3h6issjqxor5ldy7yq64qv5wk5jawqeorfhn65evoe")
	hashedURL := util.Hash(expectedRecord.URL)

	mockCache := testutil.NewMockCache(gomock.NewController(t))
	mockCache.EXPECT().ReadPost(hashedCID).Return(cache.PostRecord{URL: expectedRecord.URL}, nil) // Check for existing post record (it exists)
	mockCache.EXPECT().RefreshPost(hashedCID).Return(nil)                                         // Refresh the TTL of the referenced post
	mockCache.EXPECT().ReadURL(hashedURL).Return(existingURL, nil)                                // Check for existing URL record (it doesn't exist)

	record, url, skip, err := app.HandleQuotePost(mockCache, event)

	if err != nil {
		t.Fatal(err)
	}
	if skip {
		t.Error("unexpected event skip")
	}
	if record.Type != expectedRecord.Type {
		t.Errorf("unexpected type: %d", record.Type)
	}
	if record.DID != expectedRecord.DID {
		t.Errorf("unexpected did: %s", record.DID)
	}
	if record.URL != expectedRecord.URL {
		t.Errorf("unexpected url: %s", record.URL)
	}
	if url.Totals.Posts != 2 {
		t.Errorf("unexpected post count: %d", url.Totals.Posts)
	}
}

// Test handling a quote post that references a post that doesn't exist in the cache.
func TestHandleQuotePostWithInvalidReference(t *testing.T) {
	event, err := testutil.GetStreamEvent("post-quote.json")
	if err != nil {
		t.Fatal(err)
	}

	hashedCID := util.Hash("bafyreihhlj7nktvq3h6issjqxor5ldy7yq64qv5wk5jawqeorfhn65evoe")

	mockCache := testutil.NewMockCache(gomock.NewController(t))
	mockCache.EXPECT().ReadPost(hashedCID).Return(cache.PostRecord{}, nil) // Check for existing post record (it doesn't exist)

	_, _, skip, err := app.HandleQuotePost(mockCache, event)

	if err != nil {
		t.Fatal(err)
	}
	if !skip {
		t.Error("expected event skip")
	}
}
