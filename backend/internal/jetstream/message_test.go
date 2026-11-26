package jetstream

import (
	"os"
	"path/filepath"
	"testing"
)

// The fixtures in testdata are real Jetstream messages, copied from v1.
const testDID = "did:plc:ruzlll5u7u7pfxybmppqyxbx"

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func mustParse(t *testing.T, p *Parser, data []byte) Message {
	t.Helper()
	msg, ok, err := p.Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !ok {
		t.Fatalf("Parse: message was dropped, want kept")
	}
	return msg
}

func mustDrop(t *testing.T, p *Parser, data []byte) {
	t.Helper()
	msg, ok, err := p.Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if ok {
		t.Fatalf("Parse: kept %+v, want dropped", msg)
	}
}

func TestPostWithLinkCard(t *testing.T) {
	got := mustParse(t, NewParser(), fixture(t, "post-embed-only.json"))
	want := Message{
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
	got := mustParse(t, NewParser(), fixture(t, "post-facet-only.json"))
	if got.Kind != KindPost || got.URL != "https://tylervigen.com/the-mystery-of-the-bloomfield-bridge" {
		t.Fatalf("got %+v", got)
	}
}

// Posts without links are kept; deciding what to do with them is up to the caller.
func TestPostWithNoLink(t *testing.T) {
	got := mustParse(t, NewParser(), fixture(t, "post-no-url.json"))
	if got.Kind != KindPost || got.URL != "" {
		t.Fatalf("got %+v", got)
	}
}

func TestQuote(t *testing.T) {
	got := mustParse(t, NewParser(), fixture(t, "post-quote.json"))
	want := Message{
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
	data := []byte(`{"did":"did:plc:a","time_us":5,"kind":"commit","commit":{"operation":"create","collection":"app.bsky.feed.post","rkey":"r1",
		"record":{"$type":"app.bsky.feed.post","langs":["en"],
			"embed":{"$type":"app.bsky.embed.recordWithMedia",
				"media":{"$type":"app.bsky.embed.images","images":[]},
				"record":{"record":{"cid":"c","uri":"at://did:plc:b/app.bsky.feed.post/p1"}}}}}}`)
	got := mustParse(t, NewParser(), data)
	if got.Kind != KindQuote || got.SubjectURI != "at://did:plc:b/app.bsky.feed.post/p1" {
		t.Fatalf("got %+v", got)
	}
}

func TestLike(t *testing.T) {
	got := mustParse(t, NewParser(), fixture(t, "like.json"))
	want := Message{
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
	data := []byte(`{"did":"did:plc:a","time_us":7,"kind":"commit","commit":{"operation":"create","collection":"app.bsky.feed.repost","rkey":"r2",
		"record":{"$type":"app.bsky.feed.repost","subject":{"cid":"c","uri":"at://did:plc:b/app.bsky.feed.post/p1"}}}}`)
	got := mustParse(t, NewParser(), data)
	want := Message{TimeUS: 7, DID: "did:plc:a", RKey: "r2", Kind: KindRepost, SubjectURI: "at://did:plc:b/app.bsky.feed.post/p1"}
	if got != want {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestDropped(t *testing.T) {
	tests := map[string]string{
		"not english": `{"did":"did:plc:a","time_us":1,"kind":"commit","commit":{"operation":"create","collection":"app.bsky.feed.post","rkey":"r",
			"record":{"$type":"app.bsky.feed.post","langs":["de"],"embed":{"$type":"app.bsky.embed.external","external":{"uri":"https://example.com"}}}}}`,
		"no languages": `{"did":"did:plc:a","time_us":1,"kind":"commit","commit":{"operation":"create","collection":"app.bsky.feed.post","rkey":"r",
			"record":{"$type":"app.bsky.feed.post","embed":{"$type":"app.bsky.embed.external","external":{"uri":"https://example.com"}}}}}`,
		"non-english quote": `{"did":"did:plc:a","time_us":1,"kind":"commit","commit":{"operation":"create","collection":"app.bsky.feed.post","rkey":"r",
			"record":{"$type":"app.bsky.feed.post","langs":["ja"],"embed":{"$type":"app.bsky.embed.record","record":{"uri":"at://did:plc:b/app.bsky.feed.post/p"}}}}}`,
		"delete": `{"did":"did:plc:a","time_us":1,"kind":"commit","commit":{"operation":"delete","collection":"app.bsky.feed.like","rkey":"r"}}`,
		"update": `{"did":"did:plc:a","time_us":1,"kind":"commit","commit":{"operation":"update","collection":"app.bsky.feed.post","rkey":"r",
			"record":{"$type":"app.bsky.feed.post","langs":["en"]}}}`,
		"identity event": `{"did":"did:plc:a","time_us":1,"kind":"identity","identity":{"did":"did:plc:a","handle":"a.bsky.social"}}`,
		"other collection": `{"did":"did:plc:a","time_us":1,"kind":"commit","commit":{"operation":"create","collection":"app.bsky.graph.follow","rkey":"r",
			"record":{"$type":"app.bsky.graph.follow","subject":"did:plc:b"}}}`,
		"like with no subject": `{"did":"did:plc:a","time_us":1,"kind":"commit","commit":{"operation":"create","collection":"app.bsky.feed.like","rkey":"r",
			"record":{"$type":"app.bsky.feed.like"}}}`,
		"missing rkey": `{"did":"did:plc:a","time_us":1,"kind":"commit","commit":{"operation":"create","collection":"app.bsky.feed.like",
			"record":{"$type":"app.bsky.feed.like","subject":{"uri":"at://did:plc:b/app.bsky.feed.post/p"}}}}`,
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			mustDrop(t, NewParser(), []byte(data))
		})
	}

	t.Run("v1 invalid fixture", func(t *testing.T) {
		mustDrop(t, NewParser(), fixture(t, "invalid-event.json"))
	})
}

func TestExcludedAccount(t *testing.T) {
	p := NewParser(testDID)
	for _, name := range []string{"post-embed-only.json", "post-quote.json", "like.json"} {
		mustDrop(t, p, fixture(t, name))
	}
	// Other users are unaffected.
	mustParse(t, NewParser("did:plc:someone-else"), fixture(t, "like.json"))
}

func TestMalformedRecord(t *testing.T) {
	data := []byte(`{"did":"did:plc:a","time_us":1,"kind":"commit","commit":{"operation":"create","collection":"app.bsky.feed.like","rkey":"r",
		"record":{"$type":"app.bsky.feed.like","subject":"not-an-object"}}}`)
	if _, ok, err := NewParser().Parse(data); err == nil || ok {
		t.Fatalf("got ok=%v err=%v, want an error", ok, err)
	}
}

func TestInvalidJSON(t *testing.T) {
	_, ok, err := NewParser().Parse([]byte(`{"did":`))
	if err == nil || ok {
		t.Fatalf("got ok=%v err=%v, want an error", ok, err)
	}
}
