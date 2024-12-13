package app

import (
	"fmt"
	"net/url"
	"strings"
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

	// Convert YouTube URLs to short form.
	// Examples:
	//	- 'https://youtube.com/watch?abc123' -> 'https://youtu.be/abc123'
	if contains(YouTubeHostList, parsed.Hostname()) {
		params := parsed.Query()
		videoID := params.Get("v")
		if videoID == "" {
			return result
		}

		result = fmt.Sprintf("https://youtu.be/%s", videoID)
	}

	// Strip query parameters (with exceptions)
	index := strings.Index(result, "?")
	if !contains(QueryParamAllowList, parsed.Hostname()) && index != -1 {
		result = result[:index]
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
