package urltools

import (
	"net/url"
	"strings"
)

var explicitContentHosts = map[string]bool{
	"beacons.ai":      true,
	"yokubo.tv":       true,
	"linktr.ee":       true,
	"allmylinks.com":  true,
	"onlyfans.com":   true,
}

// Ignore determines whether to ignore a given URL, i.e. exclude it from our data.
func Ignore(input string) bool {
	return shouldIgnore(input)
}

func shouldIgnore(input string) bool {
	if input == "" {
		return true
	}

	parsed, err := url.Parse(input)
	if err != nil {
		return true
	}

	return shouldIgnoreParsed(input, parsed)
}

// shouldIgnoreParsed performs ignore checks using an already-parsed URL.
func shouldIgnoreParsed(input string, parsed *url.URL) bool {
	// Ignore insecure URLs
	if parsed.Scheme != "https" {
		return true
	}

	hostname := parsed.Hostname()

	// Ignore known image hosts
	if hostname == "media.tenor.com" {
		return true
	}

	// Ignore known bots
	if hostname == "mesonet.agron.iastate.edu" {
		return true
	}

	// Ignore known sites that *generally* share explicit content
	if explicitContentHosts[hostname] {
		return true
	}

	// Ignore links to the app itself. The purpose of this project is to track external links.
	if hostname == "bsky.app" || hostname == "go.bsky.app" || strings.HasSuffix(hostname, ".bsky.social") {
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
