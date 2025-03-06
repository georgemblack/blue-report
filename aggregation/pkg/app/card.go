package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

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
		title, imageURL, err := browserRender(cloudflareToken, cloudflareAccountID, url)
		if err != nil {
			slog.Warn(util.WrapErr("failed to get data from browser rendering", err).Error())
		}

		if result.Title == "" {
			result.Title = title
		}
		if result.ImageURL == "" {
			result.ImageURL = imageURL
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

func browserRender(cloudflareToken, cloudflareAccountID, url string) (string, string, error) {
	reqBody := BrowserRenderingRequest{
		URL: url,
		Elements: []BrowserRenderingRequestElements{
			{Selector: "meta[property=\"og:title\"]"},
			{Selector: "meta[property=\"og:image\"]"},
		},
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", util.WrapErr("failed to marshal request body", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/browser-rendering/scrape", cloudflareAccountID), bytes.NewBuffer(data))
	if err != nil {
		return "", "", util.WrapErr("failed to create request", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cloudflareToken))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", util.WrapErr("failed to send request", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("failed to get title from browser rendering: status code %s", resp.Status)
	}

	var browserRenderingResponse BrowserRenderingResponse
	if err := json.NewDecoder(resp.Body).Decode(&browserRenderingResponse); err != nil {
		return "", "", util.WrapErr("failed to decode browser rendering response", err)
	}

	if !browserRenderingResponse.Success {
		return "", "", errors.New("browswer rendering request failed")
	}

	title := ""
	imageURL := ""
	for _, result := range browserRenderingResponse.Result {
		for _, r := range result.Results {
			// Search for title result
			if result.Selector == "meta[property=\"og:title\"]" {
				for _, attr := range r.Attributes {
					if attr.Name == "content" {
						title = attr.Value
					}
				}
			}

			// Search for image result
			if result.Selector == "meta[property=\"og:image\"]" {
				for _, attr := range r.Attributes {
					if attr.Name == "content" {
						imageURL = attr.Value
					}
				}
			}
		}
	}

	return title, imageURL, nil
}
