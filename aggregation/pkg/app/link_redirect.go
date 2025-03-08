package app

import (
	"log/slog"
	"sync"

	"github.com/georgemblack/blue-report/pkg/queue"
	"github.com/georgemblack/blue-report/pkg/storage"
	"github.com/georgemblack/blue-report/pkg/urltools"
	"github.com/georgemblack/blue-report/pkg/util"
)

// ResolveLinkRedirects pulls URLs from an SQS queue that need to be normalized.
// URLs are normalized by checking for redirects. Translation rules are written to storage.
func ResolveLinkRedirects() error {
	slog.Info("starting link redirect")

	app, err := NewApp()
	if err != nil {
		return util.WrapErr("failed to create app", err)
	}
	defer app.Close()

	// Poll for messages from SQS and and process. Exit if we have read all messages.
	for {
		messages, err := app.Queue.Receive()
		if err != nil {
			return util.WrapErr("failed to receive messages", err)
		}

		if len(messages) == 0 {
			slog.Info("no messages found, exiting")
			return nil
		}

		wg := sync.WaitGroup{}
		wg.Add(len(messages))

		for _, msg := range messages {
			slog.Info("normalizing url", "url", msg.URL)
			go resolveLink(app.Storage, msg, &wg)
		}

		wg.Wait()
	}
}

func resolveLink(st Storage, msg queue.Message, wg *sync.WaitGroup) {
	defer wg.Done()

	// Normalize the URL by checking for redirects
	redirect := urltools.FindRedirect(msg.URL)
	if redirect == "" {
		slog.Debug("no redirect found for url", "url", msg.URL)
		return
	}

	// Clean the redirect URL (i.e. junk like query params may have been added)
	cleaned := urltools.Clean(redirect)

	// Write the translation to storage
	slog.Info("saving translated url", "url", cleaned)
	err := st.SaveURLTranslation(storage.URLTranslation{
		Source:      msg.URL,
		Destination: cleaned,
	})
	if err != nil {
		slog.Error("failed to save url translation", "url", msg.URL, "error", err)
	}
}
