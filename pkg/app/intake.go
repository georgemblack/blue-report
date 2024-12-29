package app

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/georgemblack/blue-report/pkg/app/util"
	"github.com/georgemblack/blue-report/pkg/cache"
	"github.com/gorilla/websocket"
)

const (
	WorkerPoolSize   = 1
	WorkerBufferSize = 10000
	JetstreamURL     = "wss://jetstream2.us-east.bsky.network/subscribe?wantedCollections=app.bsky.feed.post&wantedCollections=app.bsky.feed.repost&wantedCollections=app.bsky.feed.like"
)

type Stats struct {
	start   time.Time
	invalid int // Number of invalid events (not a post, like, or repost)
	skipped int // Number of skipped events (e.g. post with no URL)
	errors  int // Number of errors
	posts   int // Number of posts saved
	likes   int // Number of likes saved
	reposts int // Number of reposts saved
}

func (s *Stats) reset() {
	s.start = time.Now()
	s.invalid = 0
	s.skipped = 0
	s.errors = 0
	s.posts = 0
	s.likes = 0
	s.reposts = 0
}

func Intake() error {
	slog.Info("starting intake")

	// Build Cache client
	ch, err := cache.New()
	if err != nil {
		return util.WrapErr("failed to create cache client", err)
	}
	defer ch.Close()

	// Build storage client
	st, err := NewStorageClient()
	if err != nil {
		return util.WrapErr("failed to create storage client", err)
	}

	// Start worker threads
	var wg sync.WaitGroup
	wg.Add(WorkerPoolSize)
	stream := make(chan StreamEvent, WorkerPoolSize*100)
	shutdown := make(chan struct{})
	for i := 0; i < WorkerPoolSize; i++ {
		go worker(i+1, stream, shutdown, ch, st, &wg)
	}

	// Connect to Jetstream
	conn, _, err := websocket.DefaultDialer.Dial(JetstreamURL, nil)
	if err != nil {
		return util.WrapErr("failed to dial jetstream", err)
	}
	defer conn.Close()

	// Send Jetstream messages to workers
	for {
		event := StreamEvent{}
		err := conn.ReadJSON(&event)
		if err != nil {
			slog.Warn(util.WrapErr("failed to read json", err).Error())
			break
		}

		stream <- event
	}

	// Signal workers to exit, and wait for them to finish
	close(shutdown)
	wg.Wait()
	return nil
}

func worker(id int, stream chan StreamEvent, shutdown chan struct{}, ch Cache, st Storage, wg *sync.WaitGroup) {
	slog.Info(fmt.Sprintf("starting worker %d", id))
	defer wg.Done()

	// Buffer used to store events before they are flushed to storage.
	buffer := make([]StorageEventRecord, 0, WorkerBufferSize)

	stats := Stats{}
	stats.reset()

	for {
		event := StreamEvent{}
		ok := true

		select {
		case event, ok = <-stream:
			if !ok {
				slog.Error("error reading message from channel, terminating worker")
				return
			}
		case <-shutdown:
			slog.Info(fmt.Sprintf("shutting down worker %d", id))
			return
		}

		// Check whether event is a valid post, repost, or like
		if !event.valid() {
			stats.invalid++
			continue
		}

		// Handle event by type
		record := StorageEventRecord{}
		skip := false
		err := error(nil)
		if event.isPost() {
			record, skip, err = HandlePost(ch, event)
		}
		if event.isLike() || event.isRepost() {
			record, skip, err = HandleLikeOrRepost(ch, event)
		}

		if err != nil {
			slog.Warn(util.WrapErr("failed to handle event", err).Error())
			stats.errors++
			continue
		}
		if skip {
			stats.skipped++
			continue
		}

		// Update stats with event type
		if event.isPost() {
			stats.posts++
		}
		if event.isLike() {
			stats.likes++
		}
		if event.isRepost() {
			stats.reposts++
		}

		// Save event to the buffer. Once the buffer is full, write to storage.
		buffer = append(buffer, record)

		if len(buffer) >= WorkerBufferSize {
			err = st.FlushEvents(stats.start, buffer)
			if err != nil {
				slog.Warn(util.WrapErr("failed to write events", err).Error())
			} else {
				slog.Info("flushed events to storage", "posts", stats.posts, "reposts", stats.reposts, "likes", stats.likes, "skipped", stats.skipped, "invalid", stats.invalid, "errors", stats.errors, "queue", len(stream))
			}
			buffer = make([]StorageEventRecord, 0, WorkerBufferSize)
			stats.reset()
		}
	}
}

// If posts contain a URL, save it as an event in storage.
// Save the post to the cache so it can be quickly referenced for reposts and likes.
// Save the URL metadata to the cache.
func HandlePost(ch Cache, event StreamEvent) (StorageEventRecord, bool, error) {
	url, title, _, image := parse(event) // Ignore description for now

	// Filter out unwanted URLs (or posts with no URL)
	if !include(url) {
		return StorageEventRecord{}, true, nil
	}

	normalizedURL := Normalize(url)
	hashedURL := util.Hash(normalizedURL)

	// Add the post to the cache, so it can be quickly referenced by reposts and likes
	postRecord := cache.CachePostRecord{
		URL: normalizedURL,
	}
	ch.SavePost(util.Hash(event.Commit.CID), postRecord)

	// Add (or update) the URL metadata in the cache
	urlRecord := cache.CacheURLRecord{
		URL:      normalizedURL,
		Title:    title,
		ImageURL: image,
	}
	existing, err := ch.ReadURL(hashedURL)
	if err != nil {
		return StorageEventRecord{}, false, util.WrapErr("failed to read url record", err)
	}

	// Update the record if one of the following is ture:
	// 1. The existing record is empty
	// 2. The existing record is partially empty (i.e. missing fields)
	// 3. The new record is complete (all fields are present)
	if existing.MissingFields() || !urlRecord.MissingFields() {
		ch.SaveURL(hashedURL, urlRecord)
	}

	// Create and return storage event
	storageRecord := StorageEventRecord{
		Type:      event.typeOf(),
		URL:       normalizedURL,
		DID:       event.DID,
		Timestamp: time.Now(),
	}
	return storageRecord, false, nil
}

// Check if a like/repost references a post stored in the cache. If it does, save the event to storage.
func HandleLikeOrRepost(ch Cache, event StreamEvent) (StorageEventRecord, bool, error) {
	postCID := event.Commit.Record.Subject.CID
	postHash := util.Hash(postCID)
	postRecord, err := ch.ReadPost(postHash)
	if err != nil {
		return StorageEventRecord{}, false, util.WrapErr("failed to read event record", err)
	}
	if !postRecord.Valid() {
		return StorageEventRecord{}, true, nil
	}

	storageRecord := StorageEventRecord{
		Type:      event.typeOf(),
		URL:       postRecord.URL,
		DID:       event.DID,
		Timestamp: time.Now(),
	}
	return storageRecord, false, nil
}

// Intended for parsing post events.
func parse(post StreamEvent) (string, string, string, string) {
	// Search embed for URL, title, description, and image
	embed := post.Commit.Record.Embed
	if embed.Type == "app.bsky.embed.external" {
		uri := embed.External.URI
		title := embed.External.Title
		description := embed.External.Description
		image := ""

		// Add image if it exists
		thumb := embed.External.Thumb
		if thumb.Type == "blob" && thumb.MimeType == "image/jpeg" {
			image = fmt.Sprintf("https://cdn.bsky.app/img/feed_thumbnail/plain/%s/%s", post.DID, thumb.Ref.Link)
		}

		return uri, title, description, image
	}

	// Otherwise, search for link facet
	for _, facet := range post.Commit.Record.Facets {
		for _, feature := range facet.Features {
			if feature.Type == "app.bsky.richtext.facet#link" && feature.URI != "" {
				return feature.URI, "", "", ""
			}
		}
	}

	return "", "", "", ""
}

// Determine whether to include a given URL.
// Ignore known image hosts, bad websites, and gifs/images.
func include(url string) bool {
	if url == "" {
		return false
	}

	// Ignore insecure URLs
	if !strings.HasPrefix(url, "https://") {
		return false
	}

	// Ignore image hosts
	if strings.HasPrefix(url, "https://media.tenor.com") {
		return false
	}

	// Ignore known bots
	// https://mesonet.agron.iastate.edu/projects/iembot/
	if strings.HasPrefix(url, "https://mesonet.agron.iastate.edu") {
		return false
	}

	// Ignore links to the app itself
	// (The Blue Report is intended to track exteranl links)
	if strings.HasPrefix(url, "https://bsky.app") || strings.HasPrefix(url, "https://go.bsky.app") {
		return false
	}

	// Ignore gifs/images
	if strings.HasSuffix(url, ".gif") {
		return false
	}
	if strings.HasSuffix(url, ".jpg") {
		return false
	}
	if strings.HasSuffix(url, ".jpeg") {
		return false
	}
	if strings.HasSuffix(url, ".png") {
		return false
	}

	return true
}
