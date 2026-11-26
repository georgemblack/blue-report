// Package translations maps links to where they redirect, so a short link
// (e.g. nyti.ms/abc) and the article it points to are ranked as one link.
//
// Not safe for use by more than one goroutine; the state actor owns it.
package translations

import "github.com/georgemblack/blue-report/backend/internal/urltools"

// MaxHops limits how many redirects Final follows, so a loop of
// translations (a → b → a) can't go on forever.
const MaxHops = 5

type Translations struct {
	destinations map[string]string // cleaned source URL → destination URL
}

func New() *Translations {
	return &Translations{destinations: make(map[string]string)}
}

// Add records that source redirects to destination.
func (t *Translations) Add(source, destination string) {
	t.destinations[source] = destination
}

// Final follows translations from a cleaned URL to the link it ends up at.
// Each destination is cleaned again, since cleaning rules may have changed
// since it was saved. ok is false if any link along the way should be
// ignored, in which case the event shouldn't be counted.
func (t *Translations) Final(url string) (final string, ok bool) {
	if urltools.Ignore(url) {
		return "", false
	}
	for range MaxHops {
		next, found := t.destinations[url]
		if !found {
			break
		}
		next = urltools.Clean(next)
		if next == url {
			break
		}
		if urltools.Ignore(next) {
			return "", false
		}
		url = next
	}
	return url, true
}

// Len returns the number of translations.
func (t *Translations) Len() int {
	return len(t.destinations)
}
