// Package hash turns strings (post addresses, URLs, DIDs) into 64-bit keys,
// so the in-memory maps don't have to store and compare long strings.
package hash

import "hash/maphash"

// The seed is random per process. Keys are only used in memory and are
// rebuilt on startup, so they never need to match between runs.
var seed = maphash.MakeSeed()

// String returns a 64-bit hash of s.
func String(s string) uint64 {
	return maphash.String(seed, s)
}
