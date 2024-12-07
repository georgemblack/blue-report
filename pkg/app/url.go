package app

import (
	"fmt"
	"net/url"
	"strings"
)

// Normalize a URL by removing query parameters, and performing domain-specific transformations.
func Normalize(input string) string {
	result := input

	// Convert YouTube URLs to short form.
	// Examples:
	//	- 'https://youtube.com/watch?abc123' -> 'https://youtu.be/abc123'
	if hasYouTubePrefix(result) {
		parsed, err := url.Parse(input)
		if err != nil {
			return result
		}

		params := parsed.Query()
		videoID := params.Get("v")
		if videoID == "" {
			return result
		}

		result = fmt.Sprintf("https://youtu.be/%s", videoID)
	}

	// Strip query parameters
	if i := strings.Index(result, "?"); i != -1 {
		result = result[:i]
	}

	return result
}

func hasYouTubePrefix(url string) bool {
	if strings.HasPrefix(url, "https://www.youtube.com") {
		return true
	}
	if strings.HasPrefix(url, "https://youtube.com") {
		return true
	}
	if strings.HasPrefix(url, "https://m.youtube.com") {
		return true
	}
	if strings.HasPrefix(url, "https://music.youtube.com") {
		return true
	}

	return false
}
