package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/georgemblack/blue-report/pkg/queue"
	"github.com/georgemblack/blue-report/pkg/storage"
	"github.com/georgemblack/blue-report/pkg/urltools"
	"github.com/georgemblack/blue-report/pkg/util"
)

const (
	NormalizeWorkerPoolSize = 2
	NormalizeBufferSize     = 10
	NormalizeCacheSize      = 1000
)

// NormalizeLinks pulls URLs from an SQS queue that need to be normalized.
// URLs are normalized by checking for redirects. Translation rules are written to storage.
func NormalizeLinks() error {
	slog.Info("starting link normalization")

	// This process is intended to be interruptible.
	// If SIGTERM is received, the context will be cancelled and the process will exit gracefully.
	ctx, cancel := context.WithCancel(context.Background())
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGTERM)

	go func() {
		sig := <-shutdown
		slog.Info(fmt.Sprintf("received signal %s, cancelling the context", sig))
		cancel()
	}()

	app, err := NewApp()
	if err != nil {
		return util.WrapErr("failed to create app", err)
	}
	defer app.Close()

	// Start workers
	var wg sync.WaitGroup
	wg.Add(NormalizeWorkerPoolSize)
	stream := make(chan queue.Message, NormalizeBufferSize)
	for i := 0; i < NormalizeWorkerPoolSize; i++ {
		go normalizeWorker(i+1, stream, app.Storage, &wg)
	}

	// Poll for messages from SQS and add to the stream for workers to process.
	// This loop will continue until the context is cancelled, for a graceful shutdown.
	for {
		select {
		case <-ctx.Done():
			// Signal workers to shut down and wait for them to finish.
			close(stream)
			wg.Wait()
			slog.Info("all workers exited, shutting down")
			return nil
		default:
			messages, err := app.Queue.Receive()
			if err != nil {
				return util.WrapErr("failed to receive messages", err)
			}
			for _, message := range messages {
				stream <- message
			}
		}
	}
}

func normalizeWorker(id int, stream chan queue.Message, st Storage, wg *sync.WaitGroup) {
	slog.Info(fmt.Sprintf("starting worker %d", id))
	defer wg.Done()

	for {
		msg, ok := <-stream
		if !ok {
			slog.Info("worker received shutdown signal", "worker", id)
			return
		}

		slog.Debug("normalizing url", "worker", id, "url", msg.URL)

		// Normalize the URL by checking for redirects
		redirect := urltools.FindRedirect(msg.URL)
		if redirect == "" {
			slog.Debug("no redirect found for url", "url", msg.URL)
			continue
		}

		// Clean the redirect URL (i.e. junk like query params may have been added)
		cleaned := urltools.Clean(redirect)

		// Write the translation to storage
		err := st.SaveURLTranslation(storage.URLTranslation{
			Source:      msg.URL,
			Destination: cleaned,
		})
		if err != nil {
			slog.Error("failed to save url translation", "url", msg.URL, "error", err)
		}
	}
}
