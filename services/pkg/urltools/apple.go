package urltools

import "net/url"

func IsAppleNewsURL(input string) bool {
	parsed, err := url.Parse(input)
	if err != nil {
		return false
	}

	return parsed.Hostname() == "apple.news" || parsed.Hostname() == "www.apple.news"
}

func IsAppleURL(input string) bool {
	parsed, err := url.Parse(input)
	if err != nil {
		return false
	}

	return parsed.Hostname() == "apple.com" || parsed.Hostname() == "www.apple.com" || parsed.Hostname() == "apple.news" || parsed.Hostname() == "www.apple.news"
}
