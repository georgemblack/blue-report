package config

import (
	"strings"
	"testing"
)

func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestDefaultsAreLocal(t *testing.T) {
	c, err := Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{DataDir: "local-data", PublishDir: "local-data/published", LogFormat: "json"}
	if c != want {
		t.Fatalf("got %+v, want %+v", c, want)
	}
}

func TestBuckets(t *testing.T) {
	c, err := Load(env(map[string]string{
		"EVENTS_BUCKET":        "blue-report-events",
		"PUBLISH_BUCKET":       "blue-report",
		"PUBLISH_PREFIX":       "engine/",
		"R2_ACCOUNT_ID":        "acct",
		"R2_ACCESS_KEY_ID":     "id",
		"R2_SECRET_ACCESS_KEY": "secret",
		"METADATA_TABLE":       "blue-report-url-metadata",
		"REDIRECTS_TABLE":      "blue-report-redirects",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.MetadataTable != "blue-report-url-metadata" || c.RedirectsTable != "blue-report-redirects" || c.EventsBucket != "blue-report-events" || c.PublishBucket != "blue-report" || c.PublishPrefix != "engine/" || c.R2SecretAccessKey != "secret" {
		t.Fatalf("got %+v", c)
	}
}

func TestPublishBucketNeedsR2Credentials(t *testing.T) {
	_, err := Load(env(map[string]string{"PUBLISH_BUCKET": "blue-report", "R2_ACCOUNT_ID": "acct"}))
	if err == nil {
		t.Fatal("want an error")
	}
	for _, name := range []string{"R2_ACCESS_KEY_ID", "R2_SECRET_ACCESS_KEY"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q doesn't mention %s", err, name)
		}
	}
	if strings.Contains(err.Error(), "R2_ACCOUNT_ID is") {
		t.Errorf("error %q mentions a variable that was set", err)
	}
}

func TestBadLogFormat(t *testing.T) {
	if _, err := Load(env(map[string]string{"LOG_FORMAT": "yaml"})); err == nil {
		t.Fatal("want an error")
	}
}
