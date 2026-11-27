package hydrate

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"
)

// Copied from v1 (services/pkg/app/format.go).

var badPrefixes = []string{
	"BREAKING:",
	"Breaking:",
	"BREAKING NEWS:",
	"Breaking News:",
	"NEW:",
	"New:",
	"EXCLUSIVE:",
	"Exclusive:",
	"🔴",
	"💥",
}

// Shortened links as Bluesky's post editor writes them, such as
// "apnews.com/article/trum...".
var truncatedURL = regexp.MustCompile(`(www\.)?[\w.-]+\.[a-z]{2,}(/[^\s]*)?\w*\.{3}`)

// formatTitle removes siren emojis and attention-grabbing prefixes such as
// "BREAKING:" from a page title.
func formatTitle(title string) string {
	title = strings.ReplaceAll(title, "🚨", "")
	for _, prefix := range badPrefixes {
		title = strings.TrimPrefix(title, prefix)
	}
	return strings.TrimSpace(title)
}

// formatPost tidies a post's text for display: one line, no siren emojis or
// attention-grabbing prefixes, and no shortened links (the link is already
// shown above the post).
func formatPost(text string) string {
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.ReplaceAll(text, "\t", " ")
	text = strings.ReplaceAll(text, "🚨", "")
	for _, prefix := range badPrefixes {
		text = strings.TrimPrefix(text, prefix)
	}
	text = strings.Join(strings.Fields(text), " ")
	return strings.TrimSpace(truncatedURL.ReplaceAllString(text, ""))
}

// urlKey is how v1 names a link in the metadata table and in thumbnail file
// names. It must stay exactly the same so v1 and v2 share them.
func urlKey(url string) string {
	h := fnv.New64a()
	h.Write([]byte(url))
	return fmt.Sprintf("%x", h.Sum64())
}
