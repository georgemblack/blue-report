package app

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/georgemblack/blue-report/pkg/config"
	"github.com/georgemblack/blue-report/pkg/llm"
	"github.com/georgemblack/blue-report/pkg/rendering"
	"github.com/georgemblack/blue-report/pkg/util"
)

type CardMetadata struct {
	Title    string
	ImageURL string
}

// Attempt to fetch the title and image URL for a given URL.
// First attempt to use Bluesky's CardyB service, followed by Cloudflare's browser rendering APIs.
func GetCardMetadata(cfg config.Config, url string) CardMetadata {
	result := CardMetadata{}

	// Bluesky exposes a service named 'CardyB' to parse web pages for OpenGraph data.
	slog.Info("fetching card metadata from cardyb", "url", url)
	title, imageURL, err := cardyB(url)
	if err != nil {
		slog.Info(util.WrapErr("cardyb failed", err).Error(), "url", url)
	}
	result.Title = title
	result.ImageURL = imageURL

	// If CardyB fails, attemp to use Cloudflare's browser rendering APIs as a fallback.
	if result.Title == "" || result.ImageURL == "" {
		slog.Info("fetching card metadata via browser rendering", "url", url)
		elements, err := rendering.GetPageElements(cfg.CloudflareAPIToken, cfg.CloudflareAccountID, []string{"meta[property=\"og:title\"]", "meta[property=\"og:image\"]"}, url)
		if err != nil {
			slog.Warn(util.WrapErr("browser rendering failed", err).Error(), "url", url)
		}

		titles := elements.GetAttribute("meta[property=\"og:title\"]", "content")
		images := elements.GetAttribute("meta[property=\"og:image\"]", "content")

		if len(titles) == 0 && len(images) == 0 {
			slog.Info("no title or image found via browser rendering", "url", url)
		}

		if len(titles) > 0 {
			result.Title = titles[0]
		}
		if len(images) > 0 {
			result.ImageURL = images[0]
		}
	}

	// If the mime-type of the resouce a PDF document, use an LLM to generate a title.
	if result.Title == "" {
		slog.Info("checking mime-type of resource", "url", url)
		client := http.Client{
			Timeout: 3 * time.Second,
		}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			slog.Warn(util.WrapErr("failed to create request", err).Error(), "url", url)
			return result
		}
		resp, err := client.Do(req)
		if err != nil {
			slog.Warn(util.WrapErr("failed to send request", err).Error(), "url", url)
			return result
		}
		defer resp.Body.Close()

		// Check 'Content-Type' header
		mimeType := resp.Header.Get("Content-Type")

		if mimeType == "application/pdf" {
			slog.Info("found pdf document, generating title via llm", "url", url)

			title, err := llm.GetDocumentTitle(cfg.OpenAIAPIKey, resp.Body)
			if err != nil {
				slog.Warn(util.WrapErr("llm failed", err).Error(), "url", url)
				return result
			}

			result.Title = title
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
