package app

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/georgemblack/blue-report/pkg/links"
	"github.com/georgemblack/blue-report/pkg/sites"
	"github.com/georgemblack/blue-report/pkg/util"
)

func PublishLinkSnapshot(snapshot links.Snapshot) error {
	slog.Info("publishing snapshot")
	start := time.Now()

	app, err := NewApp()
	if err != nil {
		return util.WrapErr("failed to create app", err)
	}

	// Save snapshot to storage as JSON
	data, err := json.Marshal(snapshot)
	if err != nil {
		return util.WrapErr("failed to marshal snapshot", err)
	}
	err = app.Storage.PublishLinkSnapshot(data)
	if err != nil {
		return util.WrapErr("failed to publish snapshot", err)
	}

	if os.Getenv("DEBUG") == "true" {
		os.WriteFile("dist/snapshot.json", data, 0644)
	}

	slog.Info("triggering deployment")
	err = deploy(app.Config.DeployHookURL)
	if err != nil {
		return util.WrapErr("failed to deploy", err)
	}

	duration := time.Since(start)
	slog.Info("publish complete", "seconds", duration.Seconds())
	return nil
}

func PublishSiteSnapshot(snapshot sites.Snapshot) error {
	slog.Info("publishing snapshot")
	start := time.Now()

	app, err := NewApp()
	if err != nil {
		return util.WrapErr("failed to create app", err)
	}

	// Save snapshot to storage as JSON
	data, err := json.Marshal(snapshot)
	if err != nil {
		return util.WrapErr("failed to marshal snapshot", err)
	}
	err = app.Storage.PublishSiteSnapshot(data)
	if err != nil {
		return util.WrapErr("failed to publish snapshot", err)
	}

	if os.Getenv("DEBUG") == "true" {
		os.WriteFile("dist/sites.json", data, 0644)
	}

	slog.Info("triggering deployment")
	err = deploy(app.Config.DeployHookURL)
	if err != nil {
		return util.WrapErr("failed to deploy", err)
	}

	duration := time.Since(start)
	slog.Info("publish complete", "seconds", duration.Seconds())
	return nil
}

// Deploy the site on CloudFlare Pages by making an HTTP POST request to the deploy webhook.
// The deploy hook URL is considered a secret.
func deploy(hookURL string) error {
	resp, err := http.Post(hookURL, "application/json", nil)
	if err != nil {
		return util.WrapErr("failed to trigger deploy", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return util.WrapErr("failed to trigger deploy", fmt.Errorf("unexpected status code: %d", resp.StatusCode))
	}

	return nil
}
