package hydrate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/georgemblack/blue-report/backend/internal/store"
)

// card asks CardyB (the service Bluesky uses to build link previews) for a
// page's title and preview image.
func (h *Hydrator) card(ctx context.Context, pageURL string) (title, image string, err error) {
	// v1 waits a second after every call so Bluesky doesn't block us.
	defer time.Sleep(h.cardyBDelay)

	var card struct {
		Title string `json:"title"`
		Image string `json:"image"`
	}
	err = h.getJSON(ctx, h.cardyBURL+"/v1/extract?url="+url.QueryEscape(pageURL), &card)
	if err != nil {
		return "", "", fmt.Errorf("cardyb: %w", err)
	}
	return card.Title, card.Image, nil
}

// Thumbnails are stored the way v1 stores them, so the two share them:
// thumbnails/<urlKey>.<jpg|png|gif> at the top of the R2 bucket, served from
// data.theblue.report.
const (
	thumbnailFolder = "thumbnails/"
	thumbnailHost   = "https://data.theblue.report/"
)

// findThumbnail returns the address of the link's saved thumbnail, or "" if
// there isn't one.
func (h *Hydrator) findThumbnail(ctx context.Context, key string) (string, error) {
	keys, err := h.thumbnails.List(ctx, thumbnailFolder+key+".")
	if err != nil || len(keys) == 0 {
		return "", err
	}
	return thumbnailHost + keys[0], nil
}

// saveThumbnail downloads a preview image, saves a copy in R2 and returns
// the copy's address.
func (h *Hydrator) saveThumbnail(ctx context.Context, key, imageURL string) (string, error) {
	resp, err := h.get(ctx, imageURL)
	if err != nil {
		return "", fmt.Errorf("downloading thumbnail: %w", err)
	}
	defer resp.Body.Close()
	image, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("downloading thumbnail: %w", err)
	}

	// Trust the server's Content-Type, else guess from the bytes, else
	// assume JPEG (as v1 does).
	contentType := resp.Header.Get("Content-Type")
	if _, ok := imageTypes[contentType]; !ok {
		contentType = http.DetectContentType(image)
		if _, ok := imageTypes[contentType]; !ok {
			contentType = "image/jpeg"
		}
	}

	name := thumbnailFolder + key + "." + imageTypes[contentType]
	meta := store.Meta{ContentType: contentType, CacheControl: "public, max-age=28800"} // 8 hours, as in v1
	if err := h.thumbnails.Put(ctx, name, image, meta); err != nil {
		return "", fmt.Errorf("saving thumbnail: %w", err)
	}
	return thumbnailHost + name, nil
}

// The image types link previews use, and their file extensions.
var imageTypes = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/gif":  "gif",
}

// blueskyPost is the part of a post from Bluesky's API that we use.
type blueskyPost struct {
	URI    string `json:"uri"`
	Author struct {
		Handle      string `json:"handle"`
		DisplayName string `json:"displayName"`
	} `json:"author"`
	Record struct {
		Text  string   `json:"text"`
		Langs []string `json:"langs"`
	} `json:"record"`
	LikeCount int `json:"likeCount"`
}

func (p blueskyPost) english() bool {
	return slices.Contains(p.Record.Langs, "en")
}

// getPosts fetches posts from Bluesky's public API by their at:// addresses,
// 25 at a time (the most one request allows). Posts that have been deleted
// are missing from the result.
func (h *Hydrator) getPosts(ctx context.Context, uris []string) (map[string]blueskyPost, error) {
	posts := make(map[string]blueskyPost, len(uris))
	for batch := range slices.Chunk(uris, 25) {
		query := url.Values{"uris": batch}
		var resp struct {
			Posts []blueskyPost `json:"posts"`
		}
		if err := h.getJSON(ctx, h.blueskyURL+"/xrpc/app.bsky.feed.getPosts?"+query.Encode(), &resp); err != nil {
			return posts, fmt.Errorf("getting posts: %w", err)
		}
		for _, p := range resp.Posts {
			posts[p.URI] = p
		}
	}
	return posts, nil
}

// get makes a GET request and returns the response if it was a 200.
func (h *Hydrator) get(ctx context.Context, address string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GET %s: %s", address, resp.Status)
	}
	return resp, nil
}

func (h *Hydrator) getJSON(ctx context.Context, address string, v any) error {
	resp, err := h.get(ctx, address)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(v)
}
