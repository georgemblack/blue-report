package app

import (
	"testing"

	"github.com/georgemblack/blue-report/pkg/testutil"
)

// Test parsing a post with no URL at all.
func TestParsePostWithNoURL(t *testing.T) {
	bytes := testutil.GetStreamEvent("post-no-url.json")
	event := toStreamEvent(bytes)

	url, title, image := event.ParsePost()
	if url != "" {
		t.Errorf("unexpected url: %s", url)
	}
	if title != "" {
		t.Errorf("unexpected title: %s", title)
	}
	if image != "" {
		t.Errorf("unexpected image url: %s", image)
	}
}

// Test parsing a post that contains a URL, title, and thumbnail via external embed.
func TestParsePostWithExternalEmbed(t *testing.T) {
	bytes := testutil.GetStreamEvent("post-embed-only.json")
	event := toStreamEvent(bytes)

	url, title, image := event.ParsePost()
	if url != "https://tylervigen.com/the-mystery-of-the-bloomfield-bridge" {
		t.Errorf("unexpected url: %s", url)
	}
	if title != "The Mystery of the Bloomfield Bridge" {
		t.Errorf("unexpected title: %s", title)
	}
	if image != "https://cdn.bsky.app/img/feed_thumbnail/plain/did:plc:ruzlll5u7u7pfxybmppqyxbx/bafkreiasj4bgohn7rx2mhf3i4r7tdr43kuyyks6cxsgi5zuttq4274ibny" {
		t.Errorf("unexpected image url: %s", image)
	}
}

// Test parsing a post that only contains a link, but no title/thumbnail.
func TestParsePostWithFacetOnly(t *testing.T) {
	bytes := testutil.GetStreamEvent("post-facet-only.json")
	event := toStreamEvent(bytes)

	url, title, image := event.ParsePost()
	if url != "https://tylervigen.com/the-mystery-of-the-bloomfield-bridge" {
		t.Errorf("unexpected url: %s", url)
	}
	if title != "" {
		t.Errorf("unexpected title: %s", title)
	}
	if image != "" {
		t.Errorf("unexpected image url: %s", image)
	}
}
