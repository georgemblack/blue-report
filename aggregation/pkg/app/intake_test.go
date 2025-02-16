package app

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/georgemblack/blue-report/pkg/cache"
	"github.com/georgemblack/blue-report/pkg/storage"
	"github.com/georgemblack/blue-report/pkg/testutil"
	"github.com/georgemblack/blue-report/pkg/util"
	"go.uber.org/mock/gomock"
)

func TestHandlePostNoURL(t *testing.T) {
	bytes := testutil.GetTestData("post-no-url.json")
	event := toStreamEvent(bytes)

	// The cache should not be called – the event does not contain a URL
	mockCache := testutil.NewMockCache(gomock.NewController(t))
	mockCache.EXPECT().ReadPost(gomock.Any()).Times(0)
	mockCache.EXPECT().SavePost(gomock.Any(), gomock.Any()).Times(0)

	_, _, skip, err := handlePost(mockCache, event)

	if !skip {
		t.Error("expected event to be skipped")
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestHandlePostWithEmbed(t *testing.T) {
	bytes := testutil.GetTestData("post-embed-only.json")
	event := toStreamEvent(bytes)

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

	stg, url, skip, err := handlePost(mockCache, event)

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
	if stg.DID != "did:plc:ruzlll5u7u7pfxybmppqyxbx" {
		t.Errorf("unexpected did: %s", stg.DID)
	}
	if stg.Post != "at://did:plc:ruzlll5u7u7pfxybmppqyxbx/app.bsky.feed.post/3ldkcy6xjvc2l" {
		t.Errorf("unexpected at uri for post: %s", stg.Post)
	}
}

// Save a post with a URL that has already been cached. The cached URL is missing data, so it should be updated.
func TestHandlePostWithPartiallySavedURL(t *testing.T) {
	bytes := testutil.GetTestData("post-embed-only.json")
	event := toStreamEvent(bytes)

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

	stg, url, skip, err := handlePost(mockCache, event)

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
	if stg.DID != "did:plc:ruzlll5u7u7pfxybmppqyxbx" {
		t.Errorf("unexpected did: %s", stg.DID)
	}
	if stg.Post != "at://did:plc:ruzlll5u7u7pfxybmppqyxbx/app.bsky.feed.post/3ldkcy6xjvc2l" {
		t.Errorf("unexpected at uri for post: %s", stg.Post)
	}
}

// Test handling a quote post
func TestHandleQuotePost(t *testing.T) {
	bytes := testutil.GetTestData("post-quote.json")
	event := toStreamEvent(bytes)

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

	stg, url, skip, err := handleQuotePost(mockCache, event)

	if err != nil {
		t.Fatal(err)
	}
	if skip {
		t.Error("unexpected event skip")
	}
	if url.Totals.Posts != 2 {
		t.Errorf("unexpected post count: %d", url.Totals.Posts)
	}
	if stg.Type != expectedRecord.Type {
		t.Errorf("unexpected type: %d", stg.Type)
	}
	if stg.DID != expectedRecord.DID {
		t.Errorf("unexpected did: %s", stg.DID)
	}
	if stg.URL != expectedRecord.URL {
		t.Errorf("unexpected url: %s", stg.URL)
	}
	if stg.Post != "at://did:plc:ruzlll5u7u7pfxybmppqyxbx/app.bsky.feed.post/3lewu3lbitc2v" {
		t.Errorf("unexpected at uri for post: %s", stg.Post)
	}
}

// Test handling a quote post that references a post that doesn't exist in the cache.
func TestHandleQuotePostWithInvalidReference(t *testing.T) {
	bytes := testutil.GetTestData("post-quote.json")
	event := toStreamEvent(bytes)

	hashedCID := util.Hash("bafyreihhlj7nktvq3h6issjqxor5ldy7yq64qv5wk5jawqeorfhn65evoe")

	mockCache := testutil.NewMockCache(gomock.NewController(t))
	mockCache.EXPECT().ReadPost(hashedCID).Return(cache.PostRecord{}, nil) // Check for existing post record (it doesn't exist)

	_, _, skip, err := handleQuotePost(mockCache, event)

	if err != nil {
		t.Fatal(err)
	}
	if !skip {
		t.Error("expected event skip")
	}
}

// Test worker with an invalid event.
// Cache and storage APIs should not be called.
func TestWorkerWithInvalidEvent(t *testing.T) {
	bytes := testutil.GetTestData("invalid-event.json")
	event := toStreamEvent(bytes)

	stream := make(chan StreamEvent, 1)
	shutdown := make(chan struct{})
	var wg sync.WaitGroup

	// Mock cache and storage should not be called
	mockCache := testutil.NewMockCache(gomock.NewController(t))
	mockStorage := testutil.NewMockStorage(gomock.NewController(t))
	mockCache.EXPECT().ReadPost(gomock.Any()).Times(0)
	mockCache.EXPECT().SavePost(gomock.Any(), gomock.Any()).Times(0)
	mockCache.EXPECT().ReadURL(gomock.Any()).Times(0)
	mockCache.EXPECT().SaveURL(gomock.Any(), gomock.Any()).Times(0)
	mockStorage.EXPECT().FlushEvents(gomock.Any(), gomock.Any()).Times(0)

	app := App{
		Cache:   mockCache,
		Storage: mockStorage,
	}

	wg.Add(1)
	go intakeWorker(1, stream, shutdown, app, &wg)

	// Send the event to the worker
	stream <- event

	close(shutdown)
	wg.Wait()
}

// Test worker a number of events that match the buffer size, to ensure events are flushed to storage.
// TODO: This is flakey, fix it.
func TestWorkerWithFlush(t *testing.T) {
	bytes := testutil.GetTestData("post-facet-only.json")
	event := toStreamEvent(bytes)

	stream := make(chan StreamEvent, 1)
	shutdown := make(chan struct{})
	var wg sync.WaitGroup

	// Ensure mock storage is called once to flush events, with correct input.
	// Ignore calls to mock cache – outside the scope of this test.
	mockCache := testutil.NewMockCache(gomock.NewController(t))
	mockCache.EXPECT().SavePost(gomock.Any(), gomock.Any()).AnyTimes()
	mockCache.EXPECT().SaveURL(gomock.Any(), gomock.Any()).AnyTimes()
	mockCache.EXPECT().ReadURL(gomock.Any()).AnyTimes()
	mockStorage := testutil.NewMockStorage(gomock.NewController(t))
	mockStorage.EXPECT().FlushEvents(gomock.Any(), gomock.Any()).Times(1)

	app := App{
		Cache:   mockCache,
		Storage: mockStorage,
	}

	wg.Add(1)
	go intakeWorker(1, stream, shutdown, app, &wg)

	// Send the event to the worker
	for i := 0; i < EventBufferSize; i++ {
		stream <- event
	}

	close(shutdown)
	wg.Wait()
}

func toStreamEvent(bytes []byte) StreamEvent {
	var event StreamEvent
	_ = json.Unmarshal(bytes, &event)
	return event
}
