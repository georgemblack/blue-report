package app

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/georgemblack/blue-report/pkg/cache"
	"github.com/georgemblack/blue-report/pkg/queue"
	"github.com/georgemblack/blue-report/pkg/storage"
	"github.com/georgemblack/blue-report/pkg/urltools"
	"github.com/georgemblack/blue-report/pkg/util"
	"github.com/gorilla/websocket"
)

const (
	WorkerPoolSize   = 1
	StreamBufferSize = 10000
	EventBufferSize  = 10000
	ErrorThreshold   = 10
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

func newStats() Stats {
	return Stats{
		start:   time.Now(),
		invalid: 0,
		skipped: 0,
		errors:  0,
		posts:   0,
		likes:   0,
		reposts: 0,
	}
}

func Intake() error {
	slog.Info("starting intake")

	app, err := NewApp()
	if err != nil {
		return util.WrapErr("failed to create app", err)
	}
	defer app.Close()

	// Start worker threads
	var wg sync.WaitGroup
	wg.Add(WorkerPoolSize)
	stream := make(chan StreamEvent, StreamBufferSize)
	shutdown := make(chan struct{})
	for i := 0; i < WorkerPoolSize; i++ {
		go intakeWorker(i+1, stream, shutdown, app, &wg)
	}

	// Connect to Jetstream
	conn, _, err := websocket.DefaultDialer.Dial(JetstreamURL, nil)
	if err != nil {
		return util.WrapErr("failed to dial jetstream", err)
	}
	defer conn.Close()

	// Send Jetstream messages to workers
	errors := 0
	for {
		event := StreamEvent{}
		err := conn.ReadJSON(&event)
		if err != nil {
			errors++
			slog.Warn(util.WrapErr("failed to read json", err).Error())

			if errors > ErrorThreshold {
				slog.Error("encountered too many errors reading from jetstream")
				break
			}

			continue
		}

		stream <- event
	}

	// Signal workers to exit, and wait for them to finish
	close(shutdown)
	wg.Wait()
	return nil
}

// The intakeWorker is responsible for processing events from the stream. This includes:
// - Determining whether the event is valid (i.e. a post, like, or repost, and references a URL)
// - Transforming the event into a storage record (and saving to S3)
// - Updating metadata in the cache
func intakeWorker(id int, stream chan StreamEvent, shutdown chan struct{}, app App, wg *sync.WaitGroup) {
	slog.Info(fmt.Sprintf("starting worker %d", id))
	defer wg.Done()

	buffer := make([]storage.EventRecord, 0, EventBufferSize) // Aggregate records before writing to storage
	stats := newStats()

	for {
		event := StreamEvent{}
		ok := true

		select {
		case event, ok = <-stream:
			if !ok {
				slog.Error("error reading message from channel")
				continue
			}
		case <-shutdown:
			slog.Info(fmt.Sprintf("shutting down worker %d", id))
			return
		}

		// Check whether event is a valid post, repost, or like
		if !event.Valid() {
			stats.invalid++
			continue
		}

		stRecord := storage.EventRecord{}
		urlRecord := cache.URLRecord{}
		skip := false
		err := error(nil)

		if event.IsPost() && !event.IsQuotePost() {
			stRecord, urlRecord, skip, err = handlePost(app.Cache, event)
		}
		if event.IsPost() && event.IsQuotePost() {
			stRecord, urlRecord, skip, err = handleQuotePost(app.Cache, event)
		}
		if event.IsLike() || event.IsRepost() {
			stRecord, urlRecord, skip, err = handleLikeOrRepost(app.Cache, event)
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
		if event.IsPost() {
			stats.posts++
		}
		if event.IsLike() {
			stats.likes++
		}
		if event.IsRepost() {
			stats.reposts++
		}

		// Save or update the URL record to cache.
		// This record contains metadata for the URL, as well as post/repost/like counts.
		// This also has the side-effect of refreshing the TTL of the URL record.
		err = app.Cache.SaveURL(util.Hash(stRecord.URL), urlRecord)
		if err != nil {
			slog.Error(util.WrapErr("failed to save url record", err).Error())
			return
		}

		// If event is a post, and the URL appears to be shortened, send the URL to the normalization queue.
		if event.IsPost() && urltools.IsShortenedURL(stRecord.URL) {
			err = app.Queue.Send(queue.Message{URL: stRecord.URL})
			if err != nil {
				slog.Error(util.WrapErr("failed to send message to queue", err).Error())
				return
			}
		}

		// Save event to the buffer.
		// Once the buffer is full, write to storage asynchronously.
		buffer = append(buffer, stRecord)
		if len(buffer) >= EventBufferSize {
			// Create local copies of buffer & stats to prevent the reset from occurring before the flush
			localBuffer := buffer
			localStats := stats
			queue := len(stream)

			go func() {
				err = app.Storage.FlushEvents(localStats.start, localBuffer)
				if err != nil {
					slog.Warn(util.WrapErr("failed to write events", err).Error())
				} else {
					slog.Info("flushed events to storage", "posts", localStats.posts, "reposts", localStats.reposts, "likes", localStats.likes, "skipped", localStats.skipped, "invalid", localStats.invalid, "errors", localStats.errors, "queue", queue)
				}
			}()
			buffer = make([]storage.EventRecord, 0, EventBufferSize)
			stats = newStats()
		}
	}
}

// handlePost processes a 'post' stream event.
// The post is also saved to the cache to be later referenced by quote posts, reposts, and likes.
func handlePost(ch Cache, event StreamEvent) (storage.EventRecord, cache.URLRecord, bool, error) {
	url, title, image := event.ParsePost()

	// Filter out unwanted URLs (or posts with no URL)
	if urltools.Ignore(url) {
		return storage.EventRecord{}, cache.URLRecord{}, true, nil
	}

	cleanedURL := urltools.Clean(url)
	hashedURL := util.Hash(cleanedURL)

	// Add the post to the cache, so it can be quickly referenced by reposts and likes.
	post := cache.PostRecord{
		URL: cleanedURL,
	}
	ch.SavePost(util.Hash(event.Commit.CID), post)

	// Merge the old URL record (if it exists) with the new URL record, and save to cache.
	// This has the side-effect of refreshing the TTL of existing URL records.
	old, err := ch.ReadURL(hashedURL)
	if err != nil {
		return storage.EventRecord{}, cache.URLRecord{}, false, util.WrapErr("failed to read url record", err)
	}
	new := cache.URLRecord{
		Title:    title,
		ImageURL: image,
	}
	urlRecord := merge(old, new)

	// Increment the post count in the URL record
	urlRecord.Totals.Posts++

	// Create and return storage event
	stgRecord := storage.EventRecord{
		Type:      event.TypeOf(),
		URL:       cleanedURL,
		DID:       event.DID,
		Timestamp: time.Now(),
		Post:      fmt.Sprintf("at://%s/app.bsky.feed.post/%s", event.DID, event.Commit.RKey), // AT URI of the current post
	}
	return stgRecord, urlRecord, false, nil
}

// handleQuotePost processes a 'quote post' stream event.
// If the embed references a post in the cache, return a storage event and URL record to save.
func handleQuotePost(ch Cache, event StreamEvent) (storage.EventRecord, cache.URLRecord, bool, error) {
	postCID := event.Commit.Record.Embed.Record.CID
	postHash := util.Hash(postCID)
	postRecord, err := ch.ReadPost(postHash)
	if err != nil {
		return storage.EventRecord{}, cache.URLRecord{}, false, util.WrapErr("failed to read post record", err)
	}
	if !postRecord.Valid() {
		return storage.EventRecord{}, cache.URLRecord{}, true, nil
	}

	// Post & and URL records have a short TTL in the cache. Refresh the TTL of the embedded post.
	// This allows us to reduce the overall size of the cache, while still retaining popular posts.
	err = ch.RefreshPost(postHash)
	if err != nil {
		slog.Warn(util.WrapErr("failed to refresh ttl of post", err).Error())
	}

	// Find the URL record and increment the post count.
	urlHash := util.Hash(postRecord.URL)
	urlRecord, err := ch.ReadURL(urlHash)
	if err != nil {
		return storage.EventRecord{}, cache.URLRecord{}, false, util.WrapErr("failed to read url record", err)
	}
	urlRecord.Totals.Posts++

	// Create and return storage event
	stgRecord := storage.EventRecord{
		Type:      event.TypeOf(),
		URL:       postRecord.URL,
		DID:       event.DID,
		Timestamp: time.Now(),
		Post:      event.Commit.Record.Embed.Record.URI, // AT URI of the embedded post
	}
	return stgRecord, urlRecord, false, nil
}

// handleLikeOrRepost processes a 'like' or 'repost' stream event.
// If the like or repost references a post in the cache, return a storage event and URL record to save.
func handleLikeOrRepost(ch Cache, event StreamEvent) (storage.EventRecord, cache.URLRecord, bool, error) {
	postCID := event.Commit.Record.Subject.CID
	postHash := util.Hash(postCID)
	postRecord, err := ch.ReadPost(postHash)
	if err != nil {
		return storage.EventRecord{}, cache.URLRecord{}, false, util.WrapErr("failed to read event record", err)
	}
	if !postRecord.Valid() {
		return storage.EventRecord{}, cache.URLRecord{}, true, nil
	}

	// Post & and URL records have a short TTL in the cache. Refresh the TTL of the related post.
	// This allows us to reduce the overall size of the cache, while still retaining popular posts.
	err = ch.RefreshPost(postHash)
	if err != nil {
		slog.Warn(util.WrapErr("failed to refresh ttl of post", err).Error())
	}

	// Find the URL record and increment the post count.
	urlHash := util.Hash(postRecord.URL)
	urlRecord, err := ch.ReadURL(urlHash)
	if err != nil {
		return storage.EventRecord{}, cache.URLRecord{}, false, util.WrapErr("failed to read url record", err)
	}
	if event.IsRepost() {
		urlRecord.Totals.Reposts++
	}
	if event.IsLike() {
		urlRecord.Totals.Likes++
	}

	stgRecord := storage.EventRecord{
		Type:      event.TypeOf(),
		URL:       postRecord.URL,
		DID:       event.DID,
		Timestamp: time.Now(),
		Post:      event.Commit.Record.Subject.URI, // AT URI of the liked/reposted post
	}
	return stgRecord, urlRecord, false, nil
}

// Merge two URL records, returning the updated record.
func merge(old, new cache.URLRecord) cache.URLRecord {
	if old.Title == "" {
		old.Title = new.Title
	}
	if old.ImageURL == "" {
		old.ImageURL = new.ImageURL
	}

	return old
}
