package app

import (
	"fmt"
	"net/http"

	"github.com/PuerkitoBio/goquery"
)

// Make an HTTP request to a website, and parse the HTML for open grpah tags.
// Also fetch the open graph image URL.
func fetchOpenGraphData(url string) (title, description, img string, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", "", "", fmt.Errorf("error fetching URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("HTTP error: status code %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", "", "", fmt.Errorf("error parsing HTML: %w", err)
	}

	// Extract OpenGraph title and description
	title, _ = doc.Find(`meta[property="og:title"]`).Attr("content")
	description, _ = doc.Find(`meta[property="og:description"]`).Attr("content")
	image, _ := doc.Find(`meta[property="og:image"]`).Attr("content")

	return title, description, image, nil
}
