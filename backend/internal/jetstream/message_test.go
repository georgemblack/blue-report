package jetstream

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The fixtures in testdata are v1's test messages (from the owner's own
// account) converted to the v2 format, plus a few more in the same style.
const testDID = "did:plc:ruzlll5u7u7pfxybmppqyxbx"

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// commit builds a v2 commit message. record is raw JSON, or "" for none.
func commit(collection, operation, record string) []byte {
	rec := ""
	if record != "" {
		rec = `,"record":` + record
	}
	return []byte(fmt.Sprintf(`{"$type":"message","payload":{"$type":"network.bsky.jetstream.subscribeEvents#commit",
		"collection":%q,"did":"did:plc:a","operation":%q,"rkey":"r1","rev":"x","seq":7,
		"time":"2026-11-26T20:11:04.105780Z","witnessedAt":"2026-11-26T20:11:04.105780Z"%s}}`, collection, operation, rec))
}

func mustParse(t *testing.T, data []byte) Message {
	t.Helper()
	msg, ok, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !ok {
		t.Fatalf("Parse: message was dropped, want kept")
	}
	return msg
}

func mustDrop(t *testing.T, data []byte) {
	t.Helper()
	msg, ok, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if ok {
		t.Fatalf("Parse: kept %+v, want dropped", msg)
	}
}

func TestPostWithLinkCard(t *testing.T) {
	got := mustParse(t, fixture(t, "post-embed-only.json"))
	want := Message{
		Seq:        28000000001,
		TimeUS:     1734489196802654,
		DID:        testDID,
		RKey:       "3ldkcy6xjvc2l",
		Kind:       KindPost,
		SubjectURI: "at://" + testDID + "/app.bsky.feed.post/3ldkcy6xjvc2l",
		URL:        "https://tylervigen.com/the-mystery-of-the-bloomfield-bridge",
	}
	if got != want {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestPostWithLinkInText(t *testing.T) {
	got := mustParse(t, fixture(t, "post-facet-only.json"))
	if got.Kind != KindPost || got.URL != "https://tylervigen.com/the-mystery-of-the-bloomfield-bridge" {
		t.Fatalf("got %+v", got)
	}
}

// Posts without links are kept; deciding what to do with them is up to the caller.
func TestPostWithNoLink(t *testing.T) {
	got := mustParse(t, fixture(t, "post-no-url.json"))
	if got.Kind != KindPost || got.URL != "" {
		t.Fatalf("got %+v", got)
	}
}

func TestQuote(t *testing.T) {
	got := mustParse(t, fixture(t, "post-quote.json"))
	want := Message{
		Seq:        28000000004,
		TimeUS:     1736019398417668,
		DID:        testDID,
		RKey:       "3lewu3st77c2v",
		Kind:       KindQuote,
		SubjectURI: "at://" + testDID + "/app.bsky.feed.post/3lewu3lbitc2v",
	}
	if got != want {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestQuoteWithMedia(t *testing.T) {
	got := mustParse(t, fixture(t, "post-quote-with-media.json"))
	if got.Kind != KindQuote || got.SubjectURI != "at://"+testDID+"/app.bsky.feed.post/3lewu3lbitc2v" {
		t.Fatalf("got %+v", got)
	}
}

func TestLike(t *testing.T) {
	got := mustParse(t, fixture(t, "like.json"))
	want := Message{
		Seq:        28000000005,
		TimeUS:     1736019684902212,
		DID:        testDID,
		RKey:       "3lewuee7bpy2l",
		Kind:       KindLike,
		SubjectURI: "at://" + testDID + "/app.bsky.feed.post/3lewu3lbitc2v",
	}
	if got != want {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestRepost(t *testing.T) {
	got := mustParse(t, fixture(t, "repost.json"))
	if got.Kind != KindRepost || got.RKey != "3lewuf2bbbb2l" || got.SubjectURI != "at://"+testDID+"/app.bsky.feed.post/3lewu3lbitc2v" {
		t.Fatalf("got %+v", got)
	}
}

func TestTimeKeepsMicroseconds(t *testing.T) {
	got := mustParse(t, commit("app.bsky.feed.like", "create", `{"$type":"app.bsky.feed.like","subject":{"uri":"at://did:plc:b/app.bsky.feed.post/p"}}`))
	want := time.Date(2026, 11, 26, 20, 11, 4, 105780000, time.UTC).UnixMicro()
	if got.TimeUS != want {
		t.Fatalf("TimeUS = %d, want %d", got.TimeUS, want)
	}
}

func TestDropped(t *testing.T) {
	tests := map[string][]byte{
		"not english":          commit("app.bsky.feed.post", "create", `{"$type":"app.bsky.feed.post","langs":["de"],"embed":{"$type":"app.bsky.embed.external","external":{"uri":"https://example.com"}}}`),
		"no languages":         commit("app.bsky.feed.post", "create", `{"$type":"app.bsky.feed.post","embed":{"$type":"app.bsky.embed.external","external":{"uri":"https://example.com"}}}`),
		"non-english quote":    commit("app.bsky.feed.post", "create", `{"$type":"app.bsky.feed.post","langs":["ja"],"embed":{"$type":"app.bsky.embed.record","record":{"uri":"at://did:plc:b/app.bsky.feed.post/p"}}}`),
		"update":               commit("app.bsky.feed.post", "update", `{"$type":"app.bsky.feed.post","langs":["en"]}`),
		"other collection":     commit("app.bsky.graph.follow", "create", `{"$type":"app.bsky.graph.follow","subject":"did:plc:b"}`),
		"like with no subject": commit("app.bsky.feed.like", "create", `{"$type":"app.bsky.feed.like"}`),
		"delete":               fixture(t, "like-delete.json"),
		"identity event":       []byte(`{"$type":"message","payload":{"$type":"network.bsky.jetstream.subscribeEvents#identity","did":"did:plc:a","seq":1,"identity":{"handle":"a.bsky.social"}}}`),
		"missing rkey":         []byte(strings.Replace(string(commit("app.bsky.feed.like", "create", `{"$type":"app.bsky.feed.like","subject":{"uri":"at://did:plc:b/app.bsky.feed.post/p"}}`)), `"rkey":"r1",`, "", 1)),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			mustDrop(t, data)
		})
	}
}

func TestInfoNotice(t *testing.T) {
	_, ok, err := Parse(fixture(t, "info-outdated-cursor.json"))
	var info *Info
	if ok || !errors.As(err, &info) {
		t.Fatalf("got ok=%v err=%v, want an *Info error", ok, err)
	}
	if info.Name != "OutdatedCursor" || !strings.Contains(info.Message, "retention floor") {
		t.Fatalf("got %+v", info)
	}
}

func TestErrors(t *testing.T) {
	tests := map[string][]byte{
		"invalid json":     []byte(`{"$type":`),
		"v1 format":        fixture(t, "invalid-event.json"),
		"malformed record": commit("app.bsky.feed.like", "create", `{"$type":"app.bsky.feed.like","subject":"not-an-object"}`),
		"malformed time":   []byte(strings.Replace(string(commit("app.bsky.feed.like", "create", `{"subject":{"uri":"at://x"}}`)), `"time":"2026-11-26T20:11:04.105780Z"`, `"time":"yesterday"`, 1)),
		"unexpected frame": []byte(`{"$type":"error","payload":{}}`),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if _, ok, err := Parse(data); err == nil || ok {
				t.Fatalf("got ok=%v err=%v, want an error", ok, err)
			}
		})
	}
}

func TestSubscribeURL(t *testing.T) {
	got := SubscribeURL("jetstream.us-west.bsky.network", 1764187864105780)
	want := "wss://jetstream.us-west.bsky.network/xrpc/network.bsky.jetstream.subscribeEvents?" +
		"collections=app.bsky.feed.post&collections=app.bsky.feed.repost&collections=app.bsky.feed.like&cursor=1764187864105780&kinds=commit"
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	if got := SubscribeURL("h", 0); strings.Contains(got, "cursor") {
		t.Fatalf("zero cursor should be left out: %s", got)
	}
}
