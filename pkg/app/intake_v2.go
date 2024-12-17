package app

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	WorkerBufferSize = 1000
	JetstreamURLV2   = "wss://jetstream2.us-east.bsky.network/subscribe?wantedCollections=app.bsky.feed.post&wantedCollections=app.bsky.feed.repost&wantedCollections=app.bsky.feed.like"
)

func IntakeV2() error {
	slog.Info("starting intake")

	// Build Valkey vk
	vk, err := NewValkeyClient()
	if err != nil {
		return wrapErr("failed to create valkey client", err)
	}
	defer vk.Close()

	// Build storage client
	st, err := NewStorageClient()
	if err != nil {
		return wrapErr("failed to create storage client", err)
	}

	// Start worker threads
	var wg sync.WaitGroup
	wg.Add(WorkerPoolSize)
	stream := make(chan StreamEvent, WorkerPoolSize*100)
	shutdown := make(chan struct{})
	for i := 0; i < WorkerPoolSize; i++ {
		go workerV2(i+1, stream, shutdown, vk, st, &wg)
	}

	// Connect to Jetstream
	conn, _, err := websocket.DefaultDialer.Dial(JetstreamURLV2, nil)
	if err != nil {
		return wrapErr("failed to dial jetstream", err)
	}
	defer conn.Close()

	// Send Jetstream messages to workers
	for {
		event := StreamEvent{}
		err := conn.ReadJSON(&event)
		if err != nil {
			slog.Warn(wrapErr("failed to read json", err).Error())
			break
		}

		stream <- event
	}

	// Signal workers to exit, and wait for them to finish
	close(shutdown)
	wg.Wait()
	return nil
}

func workerV2(id int, stream chan StreamEvent, shutdown chan struct{}, vk Valkey, st Storage, wg *sync.WaitGroup) {
	slog.Info(fmt.Sprintf("starting worker %d", id))
	defer wg.Done()

	// Buffer used to store events before they are flushed to storage.
	buffer := make([]StorageEventRecord, 0, WorkerBufferSize)

	invalid := 0
	skipped := 0
	success := 0
	errors := 0

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
			invalid++
			continue
		}

		// Handle event by type
		record := StorageEventRecord{}
		skip := false
		err := error(nil)
		if event.isPost() {
			record, skip, err = handlePost(vk, event)
		}
		if event.isLike() || event.isRepost() {
			record, skip, err = handleLikeOrRepost(vk, event)
		}

		if err != nil {
			slog.Warn(wrapErr("failed to handle event", err).Error())
			errors++
			continue
		}
		if skip {
			skipped++
			continue
		}

		// Save event to the buffer. Once the buffer is full, write to storage.
		buffer = append(buffer, record)
		success++

		if len(buffer) >= WorkerBufferSize {
			err = st.FlushEvents(buffer)
			if err != nil {
				slog.Warn(wrapErr("failed to write events", err).Error())
			}
			buffer = make([]StorageEventRecord, 0, WorkerBufferSize)

			slog.Info("flushed events to storage", "written_events", success, "skipped_events", skipped, "invalid_events", invalid, "errors", errors)
			success = 0
			skipped = 0
			invalid = 0
			errors = 0
		}
	}
}

// If posts contain a URL, save it as an event in storage.
// Save the post to Valkey so it can be quickly referenced for reposts and likes.
// Save the URL metadata to Valkey.
func handlePost(vk Valkey, event StreamEvent) (StorageEventRecord, bool, error) {
	url, title, description, image := parse(event)

	// Filter out unwanted URLs (or posts with no URL)
	if !include(url) {
		return StorageEventRecord{}, true, nil
	}

	normalizedURL := Normalize(url)
	hashedURL := hash(normalizedURL)

	// Add the post to Valkey, so it can be quickly referenced by reposts and likes
	postRecord := VKPostRecord{
		DID:     event.DID,
		URLHash: hashedURL,
	}
	vk.SavePost(hash(event.Commit.CID), postRecord)

	// Add (or update) the URL metadata in Valkey
	urlRecord := VKURLRecord{
		URL:         normalizedURL,
		Title:       title,
		Description: description,
		ImageURL:    image,
	}
	existing, err := vk.ReadURL(hashedURL)
	if err != nil {
		return StorageEventRecord{}, false, wrapErr("failed to read url record", err)
	}

	// Update the record if one of the following is ture:
	// 1. The existing record is empty
	// 2. The existing record is partially empty (i.e. missing fields)
	// 3. The new record is complete (all fields are present)
	if existing.MissingFields() || !urlRecord.MissingFields() {
		vk.SaveURL(hashedURL, urlRecord)
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

// Check if a like/repost references a post stored in Valkey. If it does, save the event to storage.
func handleLikeOrRepost(vk Valkey, event StreamEvent) (StorageEventRecord, bool, error) {
	postCID := event.Commit.Record.Subject.CID
	postHash := hash(postCID)
	postRecord, err := vk.ReadPost(postHash)
	if err != nil {
		return StorageEventRecord{}, false, wrapErr("failed to read event record", err)
	}
	if !postRecord.Valid() {
		return StorageEventRecord{}, true, nil
	}

	storageRecord := StorageEventRecord{
		Type:      event.typeOf(),
		URL:       postRecord.URLHash,
		DID:       event.DID,
		Timestamp: time.Now(),
	}
	return storageRecord, false, nil
}
