package app

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

const (
	WorkerPoolSize   = 1
	WorkerCheckpoint = 1000 // Log a checkpoint every number of events
	JetstreamURL     = "wss://jetstream2.us-east.bsky.network/subscribe?wantedCollections=app.bsky.feed.post&wantedCollections=app.bsky.feed.repost"
)

func Intake() error {
	slog.Info("starting intake")

	// Build Valkey vk
	vk, err := NewValkeyClient()
	if err != nil {
		return wrapErr("failed to create valkey client", err)
	}
	defer vk.Close()

	// Start worker threads
	var wg sync.WaitGroup
	wg.Add(WorkerPoolSize)
	stream := make(chan StreamEvent, WorkerPoolSize*100)
	shutdown := make(chan struct{})
	for i := 0; i < WorkerPoolSize; i++ {
		go worker(i+1, stream, shutdown, vk, &wg)
	}

	// Connect to Jetstream
	conn, _, err := websocket.DefaultDialer.Dial(JetstreamURL, nil)
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

// Process posts by extracting URIs and saving them to Valkey.
func worker(id int, stream chan StreamEvent, shutdown chan struct{}, client Valkey, wg *sync.WaitGroup) {
	slog.Info(fmt.Sprintf("starting worker %d", id))
	defer wg.Done()

	successCount := 0
	skippedCount := 0
	errorCount := 0

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

		if !valid(event) {
			skippedCount++
			continue
		}

		eventRecord := EventRecord{}
		urlRecord := URLRecord{}

		// If the event is a post, save event and URL to Valkey.
		if event.isPost() {
			url, title, description, image := parse(event)

			// Filter out unwanted URLs
			if !include(url) {
				skippedCount++
				continue
			}

			// Build event record
			normalized := Normalize(url)
			hashed := hash(normalized)
			eventRecord = EventRecord{
				Type:    0,
				URLHash: hashed,
				DID:     event.DID,
			}

			// Build URL record
			urlRecord = URLRecord{
				URL:         normalized,
				Title:       title,
				Description: description,
				ImageURL:    image,
			}
		}

		// If the event is a repost, attempt to find the original post in Valkey.
		// If it exists, extract the URL and save it.
		if event.isRepost() {
			postCID := event.Commit.Record.Subject.CID
			postHash := hash(postCID)
			postRecord, err := client.ReadEvent(postHash)
			if err != nil || !postRecord.Valid() {
				skippedCount++
				continue
			}

			// Build event record
			eventRecord = EventRecord{
				Type:    1,
				URLHash: postRecord.URLHash,
				DID:     event.DID,
			}
		}

		// Save the event
		hash := hash(event.Commit.CID)
		err := client.SaveEvent(hash, eventRecord)
		if err != nil {
			errorCount++
			slog.Error(err.Error())
		} else {
			successCount++
			slog.Debug("saved event record", "worker", id, "hash", hash, "record", eventRecord)
		}

		// Save (or update) the URL record if the event is a post
		if event.isPost() {
			existing, err := client.ReadURL(eventRecord.URLHash)
			if err != nil {
				errorCount++
				slog.Error(err.Error())
				continue
			}

			// Update the record if one of the following is ture:
			// 1. The existing record is empty
			// 2. The existing record is partially empty (i.e. missing fields)
			// 3. The new record is complete (all fields are present)
			if existing.MissingFields() || urlRecord.MissingFields() {
				client.SaveURL(eventRecord.URLHash, urlRecord)
				successCount++
				slog.Debug("saved url record", "worker", id, "hash", eventRecord.URLHash, "record", urlRecord)
			}
		}

		// Log a checkpoint every number of successful of events
		if successCount >= WorkerCheckpoint || errorCount >= WorkerCheckpoint {
			slog.Info("worker checkpoint", "worker", id, "success", successCount, "error", errorCount, "skipped_events", skippedCount, "queue", len(stream))
			successCount = 0
			skippedCount = 0
			errorCount = 0
		}
	}
}

func valid(event StreamEvent) bool {
	if event.Kind != "commit" {
		return false
	}
	if event.Commit.Operation != "create" {
		return false
	}
	if !event.isPost() && !event.isRepost() {
		return false
	}
	if event.isPost() && !event.isEnglish() {
		return false
	}
	return true
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
	if strings.HasPrefix(url, "https://bsky.app") {
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
