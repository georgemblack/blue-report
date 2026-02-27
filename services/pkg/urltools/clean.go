package urltools

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var substackRegex = regexp.MustCompile(`https://open\.substack\.com/pub/([^/]+)/p/(.+)`)

// Clean performs some basic cleaning of a URL. This is done is a precursor to full normalization.
func Clean(input string) string {
	result, _ := cleanWithParsed(input)
	return result
}

// ProcessURL combines Ignore, Clean, and host extraction with a single url.Parse call.
// Returns the cleaned URL, the hostname (with www. stripped), and whether the URL should be ignored.
func ProcessURL(input string) (cleaned string, host string, ignore bool) {
	if input == "" {
		return "", "", true
	}

	// Strip fragments before parsing
	result := strings.Split(input, "#")[0]

	parsed, err := url.Parse(result)
	if err != nil {
		return "", "", true
	}

	// Run ignore checks using the parsed URL
	if shouldIgnoreParsed(input, parsed) {
		return "", "", true
	}

	// Clean the URL using the already-parsed result
	cleaned = cleanFromParsed(result, parsed)
	host = strings.TrimPrefix(parsed.Hostname(), "www.")
	return cleaned, host, false
}

func cleanWithParsed(input string) (string, *url.URL) {
	// Strip fragments
	result := strings.Split(input, "#")[0]

	// If the URL cannot be parsed, there is nothing to modify.
	parsed, err := url.Parse(result)
	if err != nil {
		return result, nil
	}

	return cleanFromParsed(result, parsed), parsed
}

func cleanFromParsed(result string, parsed *url.URL) string {
	// Strip query parameters (with exceptions)
	result = stripQueryWithExceptions(result, parsed)

	hostname := parsed.Hostname()

	// Remove mobile YouTube links.
	// Examples:
	// 	- 'https://m.youtube.com/watch?v=abc123' -> 'https://youtube.com/watch?v=abc123'
	if hostname == "m.youtube.com" {
		result = strings.Replace(result, "m.youtube.com", "www.youtube.com", 1)
	}

	// Prepend 'www', as YouTube redirects to this subdomain.
	if hostname == "youtube.com" {
		result = strings.Replace(result, "youtube.com", "www.youtube.com", 1)
	}

	// Normalize YouTube share links.
	// Examples:
	// 	- 'https://youtu.be/abc123' -> 'https://www.youtube.com/watch?v=abc123'
	if hostname == "youtu.be" {
		split := strings.Split(result, "/")
		videoID := split[len(split)-1]
		result = fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	}

	// Normalize Substack share links.
	// Examples:
	// 	- 'https://open.substack.com/pub/my-substack/p/my-article' -> 'https://my-substack.substack.com/p/my-article'
	if substackRegex.MatchString(result) {
		result = strings.Replace(result, "https://open.substack.com/pub/", "https://", 1)
		result = strings.Replace(result, "/p/", ".substack.com/p/", 1)
	}

	return result
}

func stripQuery(input string) string {
	index := strings.Index(input, "?")
	if index == -1 {
		return input
	}

	return input[:index]
}

// Some websites have query parameters that are necessary for the URL to function properly.
// This list contains the hosts, as well as the query parameters that are allowed.
var QueryParamAllowList = map[string][]string{
	"youtube.com":          {"v"},
	"m.youtube.com":        {"v"},
	"www.youtube.com":      {"v"},
	"abcnews.go.com":       {"id"},
	"commons.stmarytx.edu": {"article", "context"},
}

func stripQueryWithExceptions(full string, parsed *url.URL) string {
	// Start by stripping all query params
	result := stripQuery(full)

	// If the host is in the allow list, re-add the allowed params (if they exist)
	allowed, ok := QueryParamAllowList[parsed.Hostname()]
	if !ok {
		return result
	}

	params := parsed.Query()
	first := true
	for _, key := range allowed {
		if val, ok := params[key]; ok {
			if first {
				result += fmt.Sprintf("?%s=%s", key, val[0])
			} else {
				result += fmt.Sprintf("&%s=%s", key, val[0])
			}
			first = false
		}
	}

	return result
}
