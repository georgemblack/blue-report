package app

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Normalize a URL by removing query parameters, and performing domain-specific transformations.
func Normalize(input string) string {
	result := input

	// Convert YouTube URLs to short form.
	// Examples:
	//	- 'https://youtube.com/watch?abc123' -> 'https://youtu.be/abc123'
	if hasYouTubePrefix(result) {
		parsed, err := url.Parse(input)
		if err != nil {
			return result
		}

		params := parsed.Query()
		videoID := params.Get("v")
		if videoID == "" {
			return result
		}

		result = fmt.Sprintf("https://youtu.be/%s", videoID)
	}

	// Strip query parameters
	if i := strings.Index(result, "?"); i != -1 {
		result = result[:i]
	}

	return result
}

func hasYouTubePrefix(url string) bool {
	if strings.HasPrefix(url, "https://www.youtube.com") {
		return true
	}
	if strings.HasPrefix(url, "https://youtube.com") {
		return true
	}
	if strings.HasPrefix(url, "https://m.youtube.com") {
		return true
	}
	if strings.HasPrefix(url, "https://music.youtube.com") {
		return true
	}

	return false
}

// Make an HTTP request to a website, and parse the HTML for title and image preview.
// Use OpenGraph tags if available, otherwise fall back to HTML title tag.
func fetchURLMetadata(toFetch string) (title, img string, err error) {
	req, err := http.NewRequest(http.MethodGet, toFetch, nil)
	if err != nil {
		return "", "", fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1.1 Safari/605.1.15")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("error fetching url: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("http error: status code %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("error parsing html: %w", err)
	}

	// Extract OpenGraph title and description
	title, _ = doc.Find(`meta[property="og:title"]`).Attr("content")
	image, _ := doc.Find(`meta[property="og:image"]`).Attr("content")

	// Fall back to HTML title tag
	if title == "" {
		title = doc.Find("title").Text()
	}

	return title, image, nil
}

func hostname(fullURL string) string {
	parsed, err := url.Parse(fullURL)
	if err != nil {
		return ""
	}

	return parsed.Hostname()
}
