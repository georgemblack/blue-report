package urltools

import (
	"net/http"
	"time"

	"github.com/georgemblack/blue-report/pkg/util"
)

var redirectStatusCodes = []int{http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect}

func FindRedirect(url string) string {
	var redirect string

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 1 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return ""
	}
	if util.ContainsInt(redirectStatusCodes, resp.StatusCode) {
		redirect = resp.Header.Get("Location")
	}
	if redirect == "" {
		return ""
	}

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
