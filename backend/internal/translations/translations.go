// Package translations maps links to where they redirect, so a short link
// (e.g. nyti.ms/abc) and the article it points to are ranked as one link.
//
// Not safe for use by more than one goroutine; the state actor owns it.
package translations

type Translations struct {
	destinations map[string]string // cleaned source URL → cleaned destination URL
}

func New() *Translations {
	return &Translations{destinations: make(map[string]string)}
}

// Add records that source redirects to destination. The destination is
// where the redirects finally end, so there's only ever one lookup.
func (t *Translations) Add(source, destination string) {
	t.destinations[source] = destination
}

// Get returns where a link redirects to, or the link itself if it isn't
// known to redirect.
func (t *Translations) Get(url string) string {
	if destination, ok := t.destinations[url]; ok {
		return destination
	}
	return url
}

// Len returns the number of translations.
func (t *Translations) Len() int {
	return len(t.destinations)
}
