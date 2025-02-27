package urltools

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/georgemblack/blue-report/pkg/util"
)

// Clean performs some basic cleaning of a URL. This is done is a precursor to full normalization.
func Clean(input string) string {
	result := input

	// If the URL cannot be parsed, there is nothing to modify.
	parsed, err := url.Parse(input)
	if err != nil {
		return result
	}

	// Strip query parameters (with exceptions)
	result = stripQueryWithExceptions(input, parsed)

	// Remove mobile YouTube links.
	// Examples:
	// 	- 'https://m.youtube.com/watch?v=abc123' -> 'https://youtube.com/watch?v=abc123'
	if parsed.Hostname() == "m.youtube.com" {
		result = strings.Replace(result, "m.youtube.com", "www.youtube.com", 1)
	}

	// Normalize YouTube share links.
	// Examples:
	// 	- 'https://youtu.be/abc123' -> 'https://youtube.com/watch?v=abc123'
	if parsed.Hostname() == "youtu.be" {
		split := strings.Split(result, "/")
		videoID := split[len(split)-1]
		result = fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	}

	// Normalize Substack share links.
	// Examples:
	// 	- 'https://open.substack.com/pub/my-substack/p/my-article' -> 'https://my-substack.substack.com/p/my-article'
	regex := `https://open\.substack\.com/pub/([^/]+)/p/(.+)`
	matched, err := util.Match(regex, result)
	if err != nil {
		slog.Warn(err.Error())
	} else if matched {
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
	"m.youtube.com":   {"v"},
	"www.youtube.com": {"v"},
	"abcnews.go.com":  {"id"},
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
	for _, key := range allowed {
		if val, ok := params[key]; ok {
			result += fmt.Sprintf("?%s=%s", key, val[0])
		}
	}

	return result
}
