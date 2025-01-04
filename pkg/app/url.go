package app

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/georgemblack/blue-report/pkg/app/util"
)

// QueryParamAllowList contains domains that use query params to identify content.
// For example, ABC News uses query params to link to an article: 'https://abcnews.go.com/US/abc-news-live/story?id=12345678
var QueryParamAllowList = []string{"abcnews.go.com"}

var YouTubeHostList = []string{"www.youtube.com", "youtube.com", "m.youtube.com", "music.youtube.com"}

// Normalize a URL by removing query parameters, and performing domain-specific transformations.
func Normalize(input string) string {
	result := input
	parsed, err := url.Parse(input)
	if err != nil {
		return result
	}

	// Normalize YouTube share links.
	// Examples:
	//	- 'https://youtube.com/watch?abc123' -> 'https://youtu.be/abc123'
	if util.Contains(YouTubeHostList, parsed.Hostname()) {
		params := parsed.Query()
		videoID := params.Get("v")
		if videoID == "" {
			return result
		}

		result = fmt.Sprintf("https://youtu.be/%s", videoID)
	}

	// Strip query parameters (with exceptions)
	index := strings.Index(result, "?")
	if !util.Contains(QueryParamAllowList, parsed.Hostname()) && index != -1 {
		result = result[:index]
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

func hostname(fullURL string) string {
	parsed, err := url.Parse(fullURL)
	if err != nil {
		return ""
	}

	return parsed.Hostname()
}
