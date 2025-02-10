package app

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/georgemblack/blue-report/pkg/app/util"
	"github.com/georgemblack/blue-report/pkg/secrets"
	"github.com/georgemblack/blue-report/pkg/storage"
)

// Publish converts a report to HTML and JSON, and publishes to an S3 bucket where the site is hosted.
func Publish(snapshot Snapshot) error {
	slog.Info("starting report publish")
	start := time.Now()

	// Build storage client
	stg, err := storage.New()
	if err != nil {
		return util.WrapErr("failed to create storage client", err)
	}

	// Build secrets client
	sec, err := secrets.New()
	if err != nil {
		return util.WrapErr("failed to create secrets client", err)
	}

	// Save snapshot to storage as JSON
	data, err := json.Marshal(snapshot)
	if err != nil {
		return util.WrapErr("failed to marshal snapshot", err)
	}
	err = stg.PublishSnapshot(data)
	if err != nil {
		return util.WrapErr("failed to publish snapshot", err)
	}

	if os.Getenv("DEBUG") == "true" {
		os.WriteFile("dist/snapshot.json", data, 0644)
	}

	slog.Info("triggering deployment")
	err = deploy(sec)
	if err != nil {
		return util.WrapErr("failed to deploy", err)
	}

	duration := time.Since(start)
	slog.Info("publish complete", "seconds", duration.Seconds())
	return nil
}

// Deploy the site on CloudFlare Pages by making an HTTP POST request to the deploy webhook.
// The deploy hook URL is considered a secret.
// TODO: Hoist secrets handling up to a global config thing.
func deploy(sec Secrets) error {
	deployHookURL, err := sec.GetDeployHook()
	if err != nil {
		return util.WrapErr("failed to get deploy hook URL", err)
	}

	resp, err := http.Post(deployHookURL, "application/json", nil)
	if err != nil {
		return util.WrapErr("failed to trigger deploy", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return util.WrapErr("failed to trigger deploy", fmt.Errorf("unexpected status code: %d", resp.StatusCode))
	}

	return nil
}
