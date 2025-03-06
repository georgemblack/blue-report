package urltools

import (
	"net/url"
	"strings"
)

// Returns the hostname of a URL for display, stripping port numbers and 'www' prefix.
func Hostname(input string) string {
	parsed, err := url.Parse(input)
	if err != nil {
		return ""
	}

	hostname := parsed.Hostname()
	hostname = strings.TrimPrefix(hostname, "www.")
	return hostname
}
