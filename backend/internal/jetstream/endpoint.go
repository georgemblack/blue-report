package jetstream

import (
	"net/url"
	"strconv"
)

// Hosts are Bluesky's public Jetstream v2 servers. No authentication is
// needed for the live stream.
var Hosts = []string{
	"jetstream.us-west.bsky.network",
	"jetstream.us-east.bsky.network",
}

// Collections are the record types to ask Jetstream for.
var Collections = []string{
	"app.bsky.feed.post",
	"app.bsky.feed.repost",
	"app.bsky.feed.like",
}

// SubscribeURL returns the WebSocket address for our stream on a host.
//
// cursorUS is where to start, as a time in microseconds since the Unix
// epoch; Jetstream starts from the first event at or after it (so expect a
// little overlap on reconnect). Zero means start from now. Jetstream keeps
// about 36 hours; an older cursor starts at the oldest event it has, after
// an "OutdatedCursor" Info notice.
func SubscribeURL(host string, cursorUS int64) string {
	q := url.Values{}
	for _, c := range Collections {
		q.Add("collections", c)
	}
	q.Set("kinds", "commit")
	if cursorUS > 0 {
		q.Set("cursor", strconv.FormatInt(cursorUS, 10))
	}
	u := url.URL{
		Scheme:   "wss",
		Host:     host,
		Path:     "/xrpc/network.bsky.jetstream.subscribeEvents",
		RawQuery: q.Encode(),
	}
	return u.String()
}
