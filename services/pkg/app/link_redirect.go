package app

import (
	"log/slog"
	"sync"

	"github.com/georgemblack/blue-report/pkg/queue"
	"github.com/georgemblack/blue-report/pkg/rendering"
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
			go resolveLink(app, msg, &wg)
		}

		wg.Wait()
	}
}

func resolveLink(app App, msg queue.Message, wg *sync.WaitGroup) {
	redirect := ""
	defer wg.Done()

	// If the URL is an Apple News URL, HTTP redirects are not used.
	// Instead, user browser rendering and parse the article URL from the web page.
	if urltools.IsAppleNewsURL(msg.URL) {
		elements, err := rendering.GetPageElements(app.Config.CloudflareAPIToken, app.Config.CloudflareAccountID, []string{"a"}, msg.URL)
		if err != nil {
			slog.Error("failed to get data from browser rendering", "error", err)
			return
		}

		// Find the first none-Apple anchor tag and set the URL to the href
		hrefs := elements.GetAttribute("a", "href")
		for _, href := range hrefs {
			if !urltools.IsAppleURL(href) {
				slog.Info("found canonical url from apple news", "original", msg.URL, "canonical", href)
				redirect = href
				break
			}
		}
	} else {
		// Check for normal redirects
		redirect = urltools.FindRedirect(msg.URL)
	}

	if redirect == "" {
		slog.Debug("no redirect found for url", "url", msg.URL)
		return
	}

	// Clean the redirect URL (i.e. junk like query params may have been added)
	cleaned := urltools.Clean(redirect)

	// Write the translation to storage
	slog.Info("saving translated url", "url", cleaned)
	err := app.Storage.SaveURLTranslation(storage.URLTranslation{
		Source:      msg.URL,
		Destination: cleaned,
	})
	if err != nil {
		slog.Error("failed to save url translation", "url", msg.URL, "error", err)
	}
}
