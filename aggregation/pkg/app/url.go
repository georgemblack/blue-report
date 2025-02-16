package app

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/georgemblack/blue-report/pkg/util"
)

// QueryParamAllowList contains domains that use query params to identify content.
// For example, ABC News uses query params to link to an article: 'https://abcnews.go.com/US/abc-news-live/story?id=12345678
var QueryParamAllowList = []string{"abcnews.go.com"}

var YouTubeHostList = []string{"www.youtube.com", "youtube.com", "m.youtube.com", "music.youtube.com"}

// normalize a URL by removing query parameters, and performing domain-specific transformations.
func normalize(input string) string {
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

// Determine whether to include a given URL.
// Ignore known image hosts, bad websites, and gifs/images.
func include(linkURL string) bool {
	if linkURL == "" {
		return false
	}

	_, err := url.Parse(linkURL)
	if err != nil {
		return false
	}

	// Ignore insecure URLs
	if !strings.HasPrefix(linkURL, "https://") {
		return false
	}

	// Ignore image hosts
	if strings.HasPrefix(linkURL, "https://media.tenor.com") {
		return false
	}

	// Ignore known bots
	// https://mesonet.agron.iastate.edu/projects/iembot/
	if strings.HasPrefix(linkURL, "https://mesonet.agron.iastate.edu") {
		return false
	}

	// Ignore known sites used by bots / explicit content
	if strings.HasPrefix(linkURL, "https://beacons.ai") {
		return false
	}

	// Prevent trend manipulation.
	// Subpaths of this site are allowed, such as 'https://www.democracydocket.com/some-news-story'.
	// However, the root domain is posted frequently without referring to a specific story.
	// The intention of The Blue Report is to showcase specific stories/events.
	if linkURL == "https://www.democracydocket.com" || linkURL == "https://www.democracydocket.com/" {
		return false
	}

	// Ignore links to the app itself. The purpose of this project is to track external links.
	if strings.HasPrefix(linkURL, "https://bsky.app") || strings.HasPrefix(linkURL, "https://go.bsky.app") {
		return false
	}

	// Ignore gifs/images
	if strings.HasSuffix(linkURL, ".gif") {
		return false
	}
	if strings.HasSuffix(linkURL, ".jpg") {
		return false
	}
	if strings.HasSuffix(linkURL, ".jpeg") {
		return false
	}
	if strings.HasSuffix(linkURL, ".png") {
		return false
	}

	return true
}
