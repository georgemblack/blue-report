package stream

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// The fixtures in testdata are real v2 messages in Jetstream's wire format:
// v1's test messages (from the owner's own account) converted to v2, plus a
// few more in the same style.
const testDID = "did:plc:ruzlll5u7u7pfxybmppqyxbx"

var fixtures = []string{
	"post-embed-only.json",
	"post-facet-only.json",
	"post-no-url.json",
	"post-quote.json",
	"like.json",
	"repost.json",
	"post-quote-with-media.json",
	"like-delete.json",
}

// fakeJetstream serves the given frames to every client, then keeps the
// connection open. It returns the address to pass as a host, and receives
// the query string of each connection.
func fakeJetstream(t *testing.T, frames [][]byte) (host string, queries <-chan string) {
	t.Helper()
	q := make(chan string, 10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xrpc/network.bsky.jetstream.subscribeEvents" {
			http.NotFound(w, r)
			return
		}
		q <- r.URL.RawQuery
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{"xrpc.v1.json"}})
		if err != nil {
			return
		}
		defer conn.CloseNow()
		for _, f := range frames {
			if err := conn.Write(r.Context(), websocket.MessageText, f); err != nil {
				return
			}
		}
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	return srv.URL, q
}

func testReader(host string) *Reader {
	return &Reader{
		Hosts:              []string{host},
		Logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
		disableCompression: true,
	}
}

func loadFixtures(t *testing.T) [][]byte {
	t.Helper()
	var frames [][]byte
	for _, name := range fixtures {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		frames = append(frames, data)
	}
	return frames
}

func receive(t *testing.T, out <-chan Message, n int) []Message {
	t.Helper()
	var got []Message
	timeout := time.After(10 * time.Second)
	for len(got) < n {
		select {
		case m := <-out:
			got = append(got, m)
		case <-timeout:
			t.Fatalf("timed out after %d of %d messages: %+v", len(got), n, got)
		}
	}
	return got
}

// Runs the fixtures through the official client library and our conversion.
func TestReaderWithFixtures(t *testing.T) {
	host, queries := fakeJetstream(t, loadFixtures(t))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := make(chan Message, 100)
	done := make(chan error, 1)
	go func() { done <- testReader(host).Run(ctx, 1736019000000000, out) }()

	subject := "at://" + testDID + "/app.bsky.feed.post/3lewu3lbitc2v"
	want := []Message{
		{Seq: 28000000001, TimeUS: 1734489196802654, DID: testDID, RKey: "3ldkcy6xjvc2l", Kind: KindPost,
			SubjectURI: "at://" + testDID + "/app.bsky.feed.post/3ldkcy6xjvc2l", URL: "https://tylervigen.com/the-mystery-of-the-bloomfield-bridge"},
		{Seq: 28000000002, TimeUS: 1735507512895621, DID: testDID, RKey: "3lehxegpi522h", Kind: KindPost,
			SubjectURI: "at://" + testDID + "/app.bsky.feed.post/3lehxegpi522h", URL: "https://tylervigen.com/the-mystery-of-the-bloomfield-bridge"},
		{Seq: 28000000003, TimeUS: 1734491192590932, DID: testDID, RKey: "3ldketq4zfk2l", Kind: KindPost,
			SubjectURI: "at://" + testDID + "/app.bsky.feed.post/3ldketq4zfk2l"},
		{Seq: 28000000004, TimeUS: 1736019398417668, DID: testDID, RKey: "3lewu3st77c2v", Kind: KindQuote, SubjectURI: subject},
		{Seq: 28000000005, TimeUS: 1736019684902212, DID: testDID, RKey: "3lewuee7bpy2l", Kind: KindLike, SubjectURI: subject},
		{Seq: 28000000006, TimeUS: 1736019700000000, DID: testDID, RKey: "3lewuf2bbbb2l", Kind: KindRepost, SubjectURI: subject},
		{Seq: 28000000007, TimeUS: 1736019800000000, DID: testDID, RKey: "3lewug3bbbb2l", Kind: KindQuote, SubjectURI: subject},
		// like-delete.json is dropped.
	}
	got := receive(t, out, len(want))
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("message %d:\n got  %+v\n want %+v", i, got[i], want[i])
		}
	}

	// The library asked for our collections, commits only, from our cursor.
	q := <-queries
	for _, part := range []string{"collections=app.bsky.feed.post", "collections=app.bsky.feed.repost", "collections=app.bsky.feed.like", "kinds=commit", "cursor="} {
		if !contains(q, part) {
			t.Errorf("query %q is missing %q", q, part)
		}
	}

	// Nothing more arrives (the delete was dropped), and Run stops when cancelled.
	select {
	case m := <-out:
		t.Fatalf("unexpected extra message %+v", m)
	case <-time.After(200 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run returned %v, want context.Canceled", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run didn't stop after cancel")
	}
}

// When the channel is full, Run waits instead of dropping messages, and
// still stops when cancelled.
func TestReaderWaitsWhenChannelIsFull(t *testing.T) {
	host, _ := fakeJetstream(t, loadFixtures(t))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := make(chan Message) // unbuffered: nothing is read until we're ready
	done := make(chan error, 1)
	go func() { done <- testReader(host).Run(ctx, 0, out) }()

	time.Sleep(200 * time.Millisecond)
	if got := receive(t, out, 7); got[0].Seq != 28000000001 || got[6].Seq != 28000000007 {
		t.Fatalf("messages out of order or missing: first=%d last=%d", got[0].Seq, got[6].Seq)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Run returned %v, want context.Canceled", err)
	}
}

func TestReaderNeedsAHost(t *testing.T) {
	r := testReader("")
	r.Hosts = nil
	if err := r.Run(context.Background(), 0, make(chan Message)); err == nil {
		t.Fatal("Run with no hosts returned nil")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
