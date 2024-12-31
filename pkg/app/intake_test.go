package app_test

import (
	"testing"

	"github.com/georgemblack/blue-report/pkg/app"
	"github.com/georgemblack/blue-report/pkg/app/util"
	"github.com/georgemblack/blue-report/pkg/cache"
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

	_, skip, err := app.HandlePost(mockCache, event)

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

	expectedPost := cache.CachePostRecord{
		URL: "https://tylervigen.com/the-mystery-of-the-bloomfield-bridge",
	}
	expectedURL := cache.CacheURLRecord{
		URL:      "https://tylervigen.com/the-mystery-of-the-bloomfield-bridge",
		Title:    "The Mystery of the Bloomfield Bridge",
		ImageURL: "https://cdn.bsky.app/img/feed_thumbnail/plain/did:plc:ruzlll5u7u7pfxybmppqyxbx/bafkreiasj4bgohn7rx2mhf3i4r7tdr43kuyyks6cxsgi5zuttq4274ibny",
	}
	hashedCID := util.Hash("bafyreiehzp2ehowobuutnjsednkq24iisx2mzpdc27yuy4xztspcqid3ni")
	hashedURL := util.Hash(expectedURL.URL)

	mockCache := testutil.NewMockCache(gomock.NewController(t))
	mockCache.EXPECT().SavePost(hashedCID, expectedPost)                      // Save the post
	mockCache.EXPECT().ReadURL(hashedURL).Return(cache.CacheURLRecord{}, nil) // Check for existing URL record (it doesn't exist)
	mockCache.EXPECT().SaveURL(hashedURL, expectedURL)                        // Write a new URL record

	_, skip, err := app.HandlePost(mockCache, event)

	if skip {
		t.Error("unexpected event skip")
	}
	if err != nil {
		t.Fatal(err)
	}
}

// Save a post with a URL that has already been cached.
// The cached URL is missing data, so it should be updated.
func TestHandlePostWithPartiallySavedURL(t *testing.T) {
	event, err := testutil.GetStreamEvent("post-embed-only.json")
	if err != nil {
		t.Fatal(err)
	}

	expectedPost := cache.CachePostRecord{
		URL: "https://tylervigen.com/the-mystery-of-the-bloomfield-bridge",
	}
	existingURL := cache.CacheURLRecord{
		URL: "https://tylervigen.com/the-mystery-of-the-bloomfield-bridge",
	}
	expectedURL := cache.CacheURLRecord{
		URL:      "https://tylervigen.com/the-mystery-of-the-bloomfield-bridge",
		Title:    "The Mystery of the Bloomfield Bridge",
		ImageURL: "https://cdn.bsky.app/img/feed_thumbnail/plain/did:plc:ruzlll5u7u7pfxybmppqyxbx/bafkreiasj4bgohn7rx2mhf3i4r7tdr43kuyyks6cxsgi5zuttq4274ibny",
	}
	hashedCID := util.Hash("bafyreiehzp2ehowobuutnjsednkq24iisx2mzpdc27yuy4xztspcqid3ni")
	hashedURL := util.Hash(expectedURL.URL)

	mockCache := testutil.NewMockCache(gomock.NewController(t))
	mockCache.EXPECT().SavePost(hashedCID, expectedPost)           // Save the post
	mockCache.EXPECT().ReadURL(hashedURL).Return(existingURL, nil) // Check for existing URL record (it exists, with missing data)
	mockCache.EXPECT().SaveURL(hashedURL, expectedURL)             // Write new URL record with complete data

	_, skip, err := app.HandlePost(mockCache, event)

	if skip {
		t.Error("unexpected event skip")
	}
	if err != nil {
		t.Fatal(err)
	}
}

// Test handling a post with partial URL data that shouldn't write new data to the cache.
func TestHandlePostWithPartialURLData(t *testing.T) {
	event, err := testutil.GetStreamEvent("post-facet-only.json")
	if err != nil {
		t.Fatal(err)
	}

	expectedPost := cache.CachePostRecord{
		URL: "https://tylervigen.com/the-mystery-of-the-bloomfield-bridge",
	}
	existingURL := cache.CacheURLRecord{
		URL:      "https://tylervigen.com/the-mystery-of-the-bloomfield-bridge",
		Title:    "The Mystery of the Bloomfield Bridge",
		ImageURL: "https://cdn.bsky.app/img/feed_thumbnail/plain/did:plc:ruzlll5u7u7pfxybmppqyxbx/bafkreiasj4bgohn7rx2mhf3i4r7tdr43kuyyks6cxsgi5zuttq4274ibny",
	}
	hashedCID := util.Hash("bafyreig3mrjwh66rbiuvlpynrzmw3y72q2qrkvhocqpxf2a3ausdcmi36e")
	hashedURL := util.Hash(existingURL.URL)

	mockCache := testutil.NewMockCache(gomock.NewController(t))
	mockCache.EXPECT().SavePost(hashedCID, expectedPost)           // Save post
	mockCache.EXPECT().ReadURL(hashedURL).Return(existingURL, nil) // Check for existing URL record (it exists, with complete data)
	mockCache.EXPECT().SaveURL(hashedURL, existingURL)             // Write new URL record with unchanged data (i.e. refresh TTL)

	_, skip, err := app.HandlePost(mockCache, event)

	if skip {
		t.Error("unexpected event skip")
	}
	if err != nil {
		t.Fatal(err)
	}
}
