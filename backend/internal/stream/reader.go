package stream

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/bluesky-social/jetstream"
)

// Hosts are Bluesky's public Jetstream v2 servers. The first is used unless
// it fails. No authentication is needed for the live stream.
var Hosts = []string{
	"jetstream.us-west.bsky.network",
	"jetstream.us-east.bsky.network",
}

// Reader holds the Jetstream connection and sends the interactions we count
// to a channel. It touches no shared state.
type Reader struct {
	Hosts  []string
	Logger *slog.Logger

	// The library compresses the stream by default. Tests turn this off,
	// since their fake server doesn't support it.
	disableCompression bool
}

func NewReader(logger *slog.Logger) *Reader {
	return &Reader{Hosts: Hosts, Logger: logger}
}

// Run reads from Jetstream until ctx is cancelled or the stream fails for
// good, sending each interaction to out. If out is full, Run waits, which
// stops reading from Jetstream until there's room.
//
// cursorUS is where to start, in microseconds since the Unix epoch (the
// newest stored event time); zero means start from now. Jetstream keeps
// about 36 hours. Expect a little overlap: the event at the cursor may be
// sent again, and switching servers rewinds a few seconds.
//
// The library reconnects on its own, so Run only returns when ctx is done
// (returning ctx.Err()) or on a fatal error.
func (r *Reader) Run(ctx context.Context, cursorUS int64, out chan<- Message) error {
	if len(r.Hosts) == 0 {
		return errors.New("stream: no jetstream hosts")
	}
	opts := []jetstream.Option{
		jetstream.WithCollections(Collections),
		jetstream.WithKinds([]jetstream.Kind{jetstream.KindCommit}),
		// Resume by time rather than sequence number, so any server works.
		jetstream.WithCursorMode(jetstream.CursorTime),
		jetstream.WithLogger(r.Logger),
	}
	if len(r.Hosts) > 1 {
		opts = append(opts, jetstream.WithFailoverHosts(r.Hosts[1:]...))
	}
	if cursorUS > 0 {
		opts = append(opts, jetstream.WithLiveCursor(uint64(cursorUS)))
	}
	if r.disableCompression {
		opts = append(opts, jetstream.WithZstdCompression(false))
	}

	client, err := jetstream.Subscribe(r.Hosts[0], opts...)
	if err != nil {
		return fmt.Errorf("stream: subscribing to jetstream: %w", err)
	}
	defer client.Close()

	for batch, err := range client.Events(ctx) {
		if err != nil {
			switch {
			case errors.Is(err, jetstream.ErrFatal):
				return fmt.Errorf("stream: %w", err)
			case errors.Is(err, jetstream.ErrCursorClamped):
				r.Logger.Warn("jetstream cursor was older than the server keeps; some events were missed", "cursor_us", cursorUS, "error", err)
			default:
				r.Logger.Warn("jetstream error", "error", err)
			}
			continue
		}
		for _, ev := range batch.Events() {
			msg, ok := FromEvent(ev)
			if !ok {
				continue
			}
			select {
			case out <- msg:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return ctx.Err()
}
