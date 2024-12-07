package app

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/valkey-io/valkey-go"
)

const (
	WorkerPoolSize   = 3
	WorkerCheckpoint = 1000 // Log a checkpoint every # of events
	JetstreamURL     = "wss://jetstream2.us-east.bsky.network/subscribe?wantedCollections=app.bsky.feed.post&wantedCollections=app.bsky.feed.repost"
)

func Intake() error {
	slog.Info("starting intake")

	// Build Valkey client
	client, err := valkeyClient()
	if err != nil {
		return wrapErr("failed to create valkey client", err)
	}
	defer client.Close()

	// Start worker threads
	var wg sync.WaitGroup
	wg.Add(WorkerPoolSize)
	stream := make(chan StreamEvent, WorkerPoolSize*100)
	shutdown := make(chan struct{})
	for i := 0; i < WorkerPoolSize; i++ {
		go worker(i+1, stream, shutdown, client, &wg)
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
func worker(id int, stream chan StreamEvent, shutdown chan struct{}, client valkey.Client, wg *sync.WaitGroup) {
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

		key := event.Commit.CID
		record := InternalRecord{}

		// If the event is a post, save metadata to Valkey.
		if isPost(event) {
			url := findURL(event)
			if url == "" {
				skippedCount++
				continue
			}

			record = InternalRecord{
				Type: 0,
				URL:  url,
				DID:  event.DID,
			}
		}

		// If the event is a repost, attempt to find the original post in Valkey.
		// If it exists, extract the URI and save it.
		if isRepost(event) {
			postKey := event.Commit.Record.Subject.CID
			postRecord, err := read(client, postKey)
			if err != nil {
				skippedCount++
				continue
			}

			record = InternalRecord{
				Type: 1,
				URL:  postRecord.URL,
				DID:  event.DID,
			}
		}

		// Filter out unwanted URLs
		if !include(record.URL) {
			skippedCount++
			continue
		}

		err := save(client, key, record)
		if err != nil {
			errorCount++
			slog.Error(err.Error())
		} else {
			successCount++
		}

		slog.Debug("saved record", "worker", id, "key", key, "record", record)

		// Log a checkpoint every number of successful of events
		if successCount >= WorkerCheckpoint || errorCount >= WorkerCheckpoint {
			slog.Info("worker checkpoint", "worker", id, "success", successCount, "skipped", skippedCount, "error", errorCount)
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
	if !isPost(event) && !isRepost(event) {
		return false
	}
	if isPost(event) && !contains(event.Commit.Record.Languages, "en") {
		return false
	}
	return true
}

func isPost(event StreamEvent) bool {
	return event.Commit.Record.Type == "app.bsky.feed.post"
}

func isRepost(event StreamEvent) bool {
	return event.Commit.Record.Type == "app.bsky.feed.repost"
}

// Extract a single URL from a post. First search the facets, followed by the embed.
func findURL(post StreamEvent) string {
	for _, facet := range post.Commit.Record.Facets {
		for _, feature := range facet.Features {
			if feature.Type == "app.bsky.richtext.facet#link" && feature.URI != "" {
				return feature.URI
			}
		}
	}

	embed := post.Commit.Record.Embed
	if embed.Type == "app.bsky.embed.external" && embed.External.URI != "" {
		return embed.External.URI
	}

	return ""
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
	if strings.HasPrefix(url, "https://media.tenor.com/") {
		return false
	}

	// Ignore bad websites

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
