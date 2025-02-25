package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/sso/types"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/georgemblack/blue-report/pkg/links"
	"github.com/georgemblack/blue-report/pkg/storage"
	"github.com/georgemblack/blue-report/pkg/util"
)

func hydrateLinks(app App, agg *links.Aggregation, snapshot links.Snapshot) (links.Snapshot, error) {
	for i := range snapshot.TopDay {
		link, err := hydrateLink(app, agg, i, snapshot.TopDay[i])
		if err != nil {
			return links.Snapshot{}, util.WrapErr("failed to hydrate link", err)
		}
		snapshot.TopDay[i] = link
	}

	for i := range snapshot.TopWeek {
		link, err := hydrateLink(app, agg, i, snapshot.TopWeek[i])
		if err != nil {
			return links.Snapshot{}, util.WrapErr("failed to hydrate link", err)
		}
		snapshot.TopWeek[i] = link
	}

	// Backwards compatibility: copy the top links from the day to the general 'Links' field.
	// https://bsky.app/profile/brianell.in/post/3lhw7kyaqgc2l
	snapshot.Links = snapshot.TopDay

	return snapshot, nil
}

func hydrateLink(app App, agg *links.Aggregation, index int, link links.Link) (links.Link, error) {
	hashedURL := util.Hash(link.URL)

	// Get the stats for this link from the aggregation.
	stats := agg.Get(link.URL)

	// Fetch the URL record from the cache, which contains the title and image URL.
	record, err := app.Cache.ReadURL(hashedURL)
	if err != nil {
		return links.Link{}, util.WrapErr("failed to read url record", err)
	}

	// Fetch the thumbnail from the Bluesky CDN and store it in our S3 bucket.
	// The thumbnail ID is the hash of the URL.
	if record.ImageURL != "" {
		err := app.Storage.SaveThumbnail(hashedURL, record.ImageURL)
		if err != nil {
			slog.Warn(util.WrapErr("failed to save thumbnail", err).Error(), "url", link.URL)
		}
	}

	// Set the thumbnail ID if it exists
	exists, err := app.Storage.ThumbnailExists(hashedURL)
	if err != nil {
		slog.Warn(util.WrapErr("failed to check for thumbnail", err).Error(), "url", link.URL)
	} else if exists {
		link.ThumbnailID = hashedURL
	}

	// Fetch the title from storage.
	// If we don't have a title, use the title in the cache.
	link.Title = getTitle(app.Storage, link.URL)
	if link.Title == "" {
		link.Title = formatTitle(record.Title)
	}

	// Update storage with the latest title.
	// Even if it's empty, having the record in storage allows us to manually add a title if it is missing.
	updateTitle(app.Storage, link.URL, link.Title)

	// Set display items, such as rank, title, host, and stats
	if link.Title == "" {
		link.Title = "(No Title)"
	}
	link.Rank = index + 1
	link.PostCount = stats.WeekCount.Posts
	link.RepostCount = stats.WeekCount.Reposts
	link.LikeCount = stats.WeekCount.Likes
	link.ClickCount = clicks(link.URL)

	// Generate a list of the most popular 1-3 posts referencing the URL.
	aggregationItem := agg.Get(link.URL)
	link.RecommendedPosts = recommendedPosts(app.Bluesky, aggregationItem.TopPosts())

	slog.Debug("hydrated", "record", link)
	return link, nil
}

func getTitle(stg Storage, url string) string {
	metadata, err := stg.GetURLMetadata(url)
	if err != nil {
		var notFoundEx *types.ResourceNotFoundException
		if errors.As(err, &notFoundEx) {
			return ""
		} else {
			slog.Warn(util.WrapErr("failed to get url metadata", err).Error(), "url", url)
		}
	}

	return metadata.Title
}

func updateTitle(stg Storage, url string, title string) {
	err := stg.SaveURLMetadata(storage.URLMetadata{
		URL:   url,
		Title: title,
	})
	if err != nil {
		slog.Warn(util.WrapErr("failed to save url metadata", err).Error(), "url", url)
	}
}

// Given the AT URIs of the top posts referencing a URL, return a list of recommended posts to display to the user.
func recommendedPosts(bs Bluesky, uris []string) []links.Post {
	posts := make([]links.Post, 0)
	authors := mapset.NewSet[string]() // Track authors that already have a reocommended post

	// For each AT URI, fetch the post from the Bluesky API.
	// If the post has enough text/commentary, add it to the list of recommended posts.
	for _, uri := range uris {
		// Avoid fetching data after three posts have been selected
		if len(posts) >= 3 {
			break
		}

		postData, err := bs.GetPost(uri)
		if err != nil {
			slog.Warn(util.WrapErr("failed to get post", err).Error(), "at_uri", uri)
			continue
		}

		// In order for the post to be recommended:
		//   - It must not be empty (after formatted & removing URLs, etc)
		//   - Post must be in English (until there's multi language/region support)
		//   - Post must have at >=50 likes (to avoid junk)
		//	 - Post cannot be from an author who already has a recommended post for this link
		formatted := formatPost(postData.Record.Text)
		if formatted != "" && postData.IsEnglish() && postData.LikeCount > 50 && !authors.Contains(postData.Author.Handle) {
			posts = append(posts, links.Post{
				AtURI:    uri,
				Username: postData.Author.DisplayName,
				Handle:   postData.Author.Handle,
				Text:     formatted,
			})
			authors.Add(postData.Author.Handle)
		}
	}

	return posts
}

func formatTitle(title string) string {
	// Remove any siren emojis, they are annoying
	title = strings.ReplaceAll(title, "🚨", "")

	// Remove any sensationalist prefixes
	title = strings.TrimPrefix(title, "BREAKING: ")
	title = strings.TrimPrefix(title, "BREAKING NEWS: ")
	title = strings.TrimPrefix(title, "NEW: ")
	title = strings.TrimPrefix(title, "🔴")
	title = strings.TrimPrefix(title, "💥")

	return title
}

func formatPost(text string) string {
	urlPattern := `(www\.)?[\w.-]+\.[a-z]{2,}(/[^\s]*)?\w*\.{3}`
	re := regexp.MustCompile(urlPattern)

	// Clean up any newlines or extra whitespace
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.ReplaceAll(text, "\t", " ")

	// Remove any siren emojis, they are annoying
	text = strings.ReplaceAll(text, "🚨", "")

	// Remove any sensationalist prefixes
	text = strings.TrimPrefix(text, "BREAKING: ")
	text = strings.TrimPrefix(text, "BREAKING NEWS: ")
	text = strings.TrimPrefix(text, "NEW: ")
	text = strings.TrimPrefix(text, "🔴")
	text = strings.TrimPrefix(text, "💥")

	// Collapse all whitespace into a single space
	text = strings.Join(strings.Fields(text), " ")

	// Remove URLs from the post text, as it is redundant.
	// The Bluesky post editor frequently truncates URL, so they appear as the following:
	//  - 'www.comicsands.com/crockett-bro...'
	// 	- 'apnews.com/article/trum...
	// 	- 'www.democracydocket.com/opinion/my-o...'
	// Use regex to find URLs that match this pattern and remove them.
	cleaned := re.ReplaceAllString(text, "")
	trimmed := strings.TrimSpace(cleaned)

	return trimmed
}

// Get the number of clicks for a given URL.
func clicks(url string) int {
	resp, err := http.Get(fmt.Sprintf("https://api.theblue.report?url=%s", url))
	if err != nil {
		err = util.WrapErr("failed to get clicks", err)
		slog.Error(err.Error(), "url", url)
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("failed to get clicks", "url", url, "status", resp.StatusCode)
		return 0
	}

	var result struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		err = util.WrapErr("failed to decode clicks response", err)
		slog.Error(err.Error(), "url", url)
		return 0
	}

	return result.Count
}
