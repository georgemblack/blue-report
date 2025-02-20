package urltools

import (
	"net/url"

	mapset "github.com/deckarep/golang-set/v2"
)

var KnownShorteners = mapset.NewSet[string]("bit.ly", "buff.ly", "ow.ly", "t.co", "shorturl.at", "goo.gl", "wapo.st", "youtu.be", "tinyurl.com")

func IsShortened(input string) bool {
	parsed, err := url.Parse(input)
	if err != nil {
		return false
	}
	return KnownShorteners.Contains(parsed.Hostname())
}
