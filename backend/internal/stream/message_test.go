package stream

import (
	"testing"

	"github.com/bluesky-social/jetstream"
)

// commit builds a decoded Jetstream event, as the library delivers it.
func commit(collection string, op jetstream.Operation, record map[string]any) jetstream.Event {
	return jetstream.Event{
		DID:           "did:plc:a",
		Seq:           7,
		TimeUS:        1000,
		WitnessedAtUS: 2000,
		Kind:          jetstream.KindCommit,
		Commit: &jetstream.Commit{
			Operation:  op,
			Collection: collection,
			Rkey:       "r1",
			Record:     record,
		},
	}
}

func post(record map[string]any) jetstream.Event {
	record["$type"] = "app.bsky.feed.post"
	if _, ok := record["langs"]; !ok {
		record["langs"] = []any{"en"}
	}
	return commit("app.bsky.feed.post", jetstream.OpCreate, record)
}

func external(uri string) map[string]any {
	return map[string]any{"$type": "app.bsky.embed.external", "external": map[string]any{"uri": uri}}
}

func linkFacet(uri string) []any {
	return []any{map[string]any{"features": []any{map[string]any{"$type": "app.bsky.richtext.facet#link", "uri": uri}}}}
}

func mustKeep(t *testing.T, ev jetstream.Event) Message {
	t.Helper()
	msg, ok := FromEvent(ev)
	if !ok {
		t.Fatalf("FromEvent dropped %+v", ev)
	}
	return msg
}

func TestPost(t *testing.T) {
	got := mustKeep(t, post(map[string]any{"embed": external("https://example.com/a")}))
	want := Message{Seq: 7, TimeUS: 2000, DID: "did:plc:a", RKey: "r1", Kind: KindPost,
		SubjectURI: "at://did:plc:a/app.bsky.feed.post/r1", URL: "https://example.com/a"}
	if got != want {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestLinkCardWinsOverText(t *testing.T) {
	got := mustKeep(t, post(map[string]any{"embed": external("https://card.com"), "facets": linkFacet("https://text.com")}))
	if got.URL != "https://card.com" {
		t.Fatalf("URL = %q", got.URL)
	}
}

func TestEmptyLinkCardFallsBackToText(t *testing.T) {
	got := mustKeep(t, post(map[string]any{"embed": external(""), "facets": linkFacet("https://text.com")}))
	if got.URL != "https://text.com" {
		t.Fatalf("URL = %q", got.URL)
	}
}

func TestOtherFacetsAreSkipped(t *testing.T) {
	facets := []any{
		map[string]any{"features": []any{map[string]any{"$type": "app.bsky.richtext.facet#mention", "did": "did:plc:b"}}},
		map[string]any{"features": []any{map[string]any{"$type": "app.bsky.richtext.facet#link", "uri": "https://second.com"}}},
	}
	if got := mustKeep(t, post(map[string]any{"facets": facets})); got.URL != "https://second.com" {
		t.Fatalf("URL = %q", got.URL)
	}
}

func TestFallsBackToTimeUS(t *testing.T) {
	ev := post(map[string]any{})
	ev.WitnessedAtUS = 0
	if got := mustKeep(t, ev); got.TimeUS != 1000 {
		t.Fatalf("TimeUS = %d, want 1000", got.TimeUS)
	}
}

func TestLikeAndRepost(t *testing.T) {
	subject := map[string]any{"subject": map[string]any{"uri": "at://did:plc:b/app.bsky.feed.post/p"}}
	if got := mustKeep(t, commit("app.bsky.feed.like", jetstream.OpCreate, subject)); got.Kind != KindLike || got.SubjectURI != "at://did:plc:b/app.bsky.feed.post/p" {
		t.Fatalf("like: %+v", got)
	}
	if got := mustKeep(t, commit("app.bsky.feed.repost", jetstream.OpCreate, subject)); got.Kind != KindRepost {
		t.Fatalf("repost: %+v", got)
	}
}

func TestDropped(t *testing.T) {
	quote := map[string]any{"$type": "app.bsky.embed.record", "record": map[string]any{"uri": "at://did:plc:b/app.bsky.feed.post/p"}}
	missingRkey := post(map[string]any{})
	missingRkey.Commit.Rkey = ""
	identity := jetstream.Event{DID: "did:plc:a", Kind: jetstream.KindIdentity, Identity: &jetstream.Identity{DID: "did:plc:a"}}

	tests := map[string]jetstream.Event{
		"not english":            post(map[string]any{"langs": []any{"de"}, "embed": external("https://example.com")}),
		"no languages":           post(map[string]any{"langs": nil}),
		"languages not a list":   post(map[string]any{"langs": "en"}),
		"non-english quote":      post(map[string]any{"langs": []any{"ja"}, "embed": quote}),
		"update":                 commit("app.bsky.feed.post", jetstream.OpUpdate, map[string]any{"langs": []any{"en"}}),
		"delete":                 commit("app.bsky.feed.like", jetstream.OpDelete, nil),
		"other collection":       commit("app.bsky.graph.follow", jetstream.OpCreate, map[string]any{"subject": "did:plc:b"}),
		"like with no subject":   commit("app.bsky.feed.like", jetstream.OpCreate, map[string]any{}),
		"subject not an object":  commit("app.bsky.feed.like", jetstream.OpCreate, map[string]any{"subject": "at://x"}),
		"missing rkey":           missingRkey,
		"identity event":         identity,
		"commit kind, no commit": {DID: "did:plc:a", Kind: jetstream.KindCommit},
	}
	for name, ev := range tests {
		t.Run(name, func(t *testing.T) {
			if msg, ok := FromEvent(ev); ok {
				t.Fatalf("kept %+v, want dropped", msg)
			}
		})
	}
}
