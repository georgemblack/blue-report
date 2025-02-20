package urltools

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/georgemblack/blue-report/pkg/util"
)

var redirectStatusCodes = []int{http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect}

func FindRedirect(input string) string {
	var redirect string

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 1 * time.Second,
	}

	resp, err := client.Get(input)
	if err != nil {
		return ""
	}
	if util.ContainsInt(redirectStatusCodes, resp.StatusCode) {
		redirect = resp.Header.Get("Location")
	}
	if redirect == "" {
		return ""
	}

	// Trim the port number if it is included in the redirect URL.
	// Patreon's redirect does this, i.e. 'https://patreon.com/george' -> 'https://www.patreon.com:443/george'
	parsed, err := url.Parse(redirect)
	if err != nil {
		return ""
	}
	parsed.Host = strings.TrimSuffix(parsed.Host, ":443")
	redirect = parsed.String()

	// Trim the fragment identifier (everything after the '#').
	redirect = strings.Split(redirect, "#")[0]

	// Check the new URL for a second redirect.
	// If a second redirect exists, return nothing – it is likely a scummy link! Bastards.
	resp, err = client.Get(redirect)
	if err != nil {
		// Could be a valid endpoint/website that's blocking us, that's fine (cough cough, Washington Post)
		return redirect
	}
	if util.ContainsInt(redirectStatusCodes, resp.StatusCode) {
		return ""
	}

	return redirect
}
