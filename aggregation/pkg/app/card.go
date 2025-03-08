package app

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/georgemblack/blue-report/pkg/rendering"
	"github.com/georgemblack/blue-report/pkg/util"
)

type CardMetadata struct {
	Title    string
	ImageURL string
}

// Attempt to fetch the title and image URL for a given URL.
// First attempt to use Bluesky's CardyB service, followed by Cloudflare's browser rendering APIs.
func GetCardMetadata(cloudflareToken, cloudflareAccountID, url string) CardMetadata {
	result := CardMetadata{}

	// CardyB
	title, imageURL, err := cardyB(url)
	if err != nil {
		slog.Warn(util.WrapErr("failed to get title from cardyb, falling back to browser rendering", err).Error())
	}
	result.Title = title
	result.ImageURL = imageURL

	// Cloudflare browser rendering
	if result.Title == "" || result.ImageURL == "" {
		elements, err := rendering.GetPageElements(cloudflareToken, cloudflareAccountID, []string{"meta[property=\"og:title\"]", "meta[property=\"og:image\"]"}, url)
		if err != nil {
			slog.Warn(util.WrapErr("failed to get data from browser rendering", err).Error())
		}

		titles := elements.GetAttribute("meta[property=\"og:title\"]", "content")
		images := elements.GetAttribute("meta[property=\"og:image\"]", "content")

		if len(titles) > 0 {
			result.Title = titles[0]
		}
		if len(images) > 0 {
			result.ImageURL = images[0]
		}
	}

	return result
}

type CardyB struct {
	Title string `json:"title"`
	Image string `json:"image"`
}

// Fetch metadata for a given URL via Bluesky's CardyB service.
func cardyB(url string) (string, string, error) {
	resp, err := http.Get("https://cardyb.bsky.app/v1/extract?url=" + url)
	if err != nil {
		return "", "", util.WrapErr("failed to get title from cardyb", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", "", errors.New("failed to get title from cardyb: status code " + resp.Status)
	}

	var cardyB CardyB
	if err := json.NewDecoder(resp.Body).Decode(&cardyB); err != nil {
		return "", "", util.WrapErr("failed to decode cardyb response", err)
	}

	// Please don't block me, Bluesky <3
	time.Sleep(1 * time.Second)

	return cardyB.Title, cardyB.Image, nil
}

type BrowserRenderingRequest struct {
	URL      string                            `json:"url"`
	Elements []BrowserRenderingRequestElements `json:"elements"`
}

type BrowserRenderingRequestElements struct {
	Selector string `json:"selector"`
}

type BrowserRenderingResponse struct {
	Success bool `json:"success"`
	Result  []struct {
		Results []struct {
			Attributes []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"attributes"`
			Text string `json:"text"`
		} `json:"results"`
		Selector string `json:"selector"`
	} `json:"result"`
}
