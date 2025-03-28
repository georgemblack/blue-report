package urltools

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/georgemblack/blue-report/pkg/util"
)

var redirectStatusCodes = []int{http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect}

// Sites where a redirect should not be followed.
// i.e. Links to standard nature.com articles redirect to a redirect service, which then redirect back to the article. This is stupid.
var noRedirectHosts = []string{"www.nature.com"}

// FindRedirect attempts to find the destination URL from a given source URL. If no redirect is found, an empty string is returned.
// Up to two redirects are followed. (Anything more than that is likely a scummy link.)
func FindRedirect(input string) string {
	var result string

	// Check if host should be ignored
	parsed, err := url.Parse(input)
	if err == nil {
		if util.ContainsStr(noRedirectHosts, parsed.Hostname()) {
			return ""
		}
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 1 * time.Second,
	}

	// Check for a first redirect
	resp, err := client.Get(input)
	if err != nil {
		// A valid website could be blocking us. Assume no redirect.
		return ""
	}
	if !util.ContainsInt(redirectStatusCodes, resp.StatusCode) || resp.Header.Get("Location") == "" {
		return ""
	}
	result = resolveLocation(input, resp.Header.Get("Location"))

	// Check for a second redirect
	resp, err = client.Get(result)
	if err != nil {
		// A valid website could be blocking us. Return the inital result.
		return result
	}
	if !util.ContainsInt(redirectStatusCodes, resp.StatusCode) || resp.Header.Get("Location") == "" {
		return result
	}
	return resolveLocation(result, resp.Header.Get("Location"))
}

// Given a URL, and the 'Location' header of its redirect, return the final destination URL. Examples:
//   - Absolute URIs: 'https://theblue.report', 'https://theblue.report/something' -> 'https://theblue.report/something'
//   - Absolute Path: 'https://theblue.report', '/something' -> 'https://theblue.report/something'
//   - Relative Path: 'https://theblue.report/something', 'else' -> 'https://theblue.report/something/else'
func resolveLocation(originalURL, location string) string {
	orig, err := url.Parse(originalURL)
	if err != nil {
		return originalURL
	}
	loc, err := url.Parse(location)
	if err != nil {
		return originalURL
	}

	if loc.IsAbs() {
		return loc.String()
	}

	// If the location is prefixed with a '/', it is an absolute path
	if strings.HasPrefix(location, "/") {
		orig.Path = location
		return orig.String()
	}

	// Otherwise, assume this is a relative path. Yikes!
	if strings.HasSuffix(originalURL, "/") {
		return fmt.Sprintf("%s%s", originalURL, location)
	}
	return fmt.Sprintf("%s/%s", originalURL, location)
}
