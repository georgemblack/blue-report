package urltools

import (
	"net/url"
	"strings"
)

// Determine whether to ignore a given URL, i.e. exclude it from our data.
func Ignore(input string) bool {
	if input == "" {
		return true
	}

	// Ignore URLs that can't be properly parsed
	parsed, err := url.Parse(input)
	if err != nil {
		return true
	}

	// Ignore insecure URLs
	if parsed.Scheme != "https" {
		return true
	}

	// Ignore known image hosts
	if parsed.Hostname() == "media.tenor.com" {
		return true
	}

	// Ignore known bots
	if parsed.Hostname() == "mesonet.agron.iastate.edu" {
		return true
	}

	// Ignore known sites that generally share explicit content
	if parsed.Hostname() == "beacons.ai" || parsed.Hostname() == "yokubo.tv" || parsed.Hostname() == "linktr.ee" {
		return true
	}

	// Ignore links to the app itself. The purpose of this project is to track external links.
	if parsed.Hostname() == "bsky.app" || parsed.Hostname() == "go.bsky.app" || strings.HasSuffix(parsed.Hostname(), ".bsky.social") {
		return true
	}

	// Ignore gifs/images
	if strings.HasSuffix(input, ".gif") {
		return true
	}
	if strings.HasSuffix(input, ".jpg") {
		return true
	}
	if strings.HasSuffix(input, ".jpeg") {
		return true
	}
	if strings.HasSuffix(input, ".png") {
		return true
	}

	return false
}
