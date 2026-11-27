// Package config reads the engine's settings from environment variables.
// With none set, everything runs locally: events and published files go to
// folders on disk instead of S3 and R2.
package config

import (
	"errors"
	"fmt"
)

type Config struct {
	// Where event files are stored: an S3 bucket if EVENTS_BUCKET is set,
	// otherwise the DATA_DIR folder.
	EventsBucket string
	DataDir      string

	// Where top-links.json and other public files go: an R2 bucket if
	// PUBLISH_BUCKET is set, otherwise the PUBLISH_DIR folder. Every key is
	// put under PUBLISH_PREFIX (e.g. "engine/"), so the engine can share a
	// bucket without overwriting anything else in it.
	PublishBucket string
	PublishPrefix string
	PublishDir    string

	// R2 credentials, required when PUBLISH_BUCKET is set.
	R2AccountID       string
	R2AccessKeyID     string
	R2SecretAccessKey string

	// The DynamoDB table of link titles, shared with v1
	// (blue-report-url-metadata). Without it, titles are only kept in memory.
	MetadataTable string

	LogFormat string // "json" (default) or "text"
}

// Load reads the configuration using getenv (normally os.Getenv).
func Load(getenv func(string) string) (Config, error) {
	or := func(key, fallback string) string {
		if v := getenv(key); v != "" {
			return v
		}
		return fallback
	}
	c := Config{
		EventsBucket:      getenv("EVENTS_BUCKET"),
		DataDir:           or("DATA_DIR", "local-data"),
		PublishBucket:     getenv("PUBLISH_BUCKET"),
		PublishPrefix:     getenv("PUBLISH_PREFIX"),
		PublishDir:        or("PUBLISH_DIR", "local-data/published"),
		R2AccountID:       getenv("R2_ACCOUNT_ID"),
		R2AccessKeyID:     getenv("R2_ACCESS_KEY_ID"),
		R2SecretAccessKey: getenv("R2_SECRET_ACCESS_KEY"),
		MetadataTable:     getenv("METADATA_TABLE"),
		LogFormat:         or("LOG_FORMAT", "json"),
	}

	if c.PublishBucket != "" {
		var missing []error
		for name, v := range map[string]string{
			"R2_ACCOUNT_ID":        c.R2AccountID,
			"R2_ACCESS_KEY_ID":     c.R2AccessKeyID,
			"R2_SECRET_ACCESS_KEY": c.R2SecretAccessKey,
		} {
			if v == "" {
				missing = append(missing, fmt.Errorf("%s is required when PUBLISH_BUCKET is set", name))
			}
		}
		if len(missing) > 0 {
			return Config{}, fmt.Errorf("config: %w", errors.Join(missing...))
		}
	}
	if c.LogFormat != "json" && c.LogFormat != "text" {
		return Config{}, fmt.Errorf("config: LOG_FORMAT must be json or text, not %q", c.LogFormat)
	}
	return c, nil
}
