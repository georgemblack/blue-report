package app

import (
	"errors"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/service/sso/types"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/georgemblack/blue-report/pkg/links"
	"github.com/georgemblack/blue-report/pkg/storage"
	"github.com/georgemblack/blue-report/pkg/util"
)

func hydrateLinks(app App, agg *links.Aggregation, snapshot links.Snapshot) (links.Snapshot, error) {
	for i := range snapshot.TopHour {
		link, err := hydrateLink(app, agg, i, snapshot.TopHour[i])
		if err != nil {
			return links.Snapshot{}, util.WrapErr("failed to hydrate link", err)
		}
		snapshot.TopHour[i] = link
	}

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
	stats := agg.Get(link.URL)

	// Check whether we have a thumbnail
	thumbnailExists, err := app.Storage.ThumbnailExists(hashedURL)
	if err != nil {
		slog.Warn(util.WrapErr("failed to check for thumbnail", err).Error(), "url", link.URL)
	}
	if thumbnailExists {
		link.ThumbnailID = hashedURL
	}

	// Check whether we have a title
	titleExists := false
	if link.Title = getTitle(app.Storage, link.URL); link.Title != "" {
		titleExists = true
	}

	// If either title or thumbnail is missing, fetch from CardyB & store
	if !thumbnailExists || !titleExists {
		metadata := GetCardMetadata(app.Config.CloudflareAPIToken, app.Config.CloudflareAccountID, link.URL)

		// Save title
		if !titleExists && metadata.Title != "" {
			link.Title = formatTitle(metadata.Title)
			updateTitle(app.Storage, link.URL, link.Title)
		}

		// Save thumbnail
		if !thumbnailExists && metadata.ImageURL != "" {
			err := app.Storage.SaveThumbnail(hashedURL, metadata.ImageURL)
			if err != nil {
				slog.Warn(util.WrapErr("failed to save thumbnail", err).Error(), "url", link.URL)
			} else {
				link.ThumbnailID = hashedURL
			}
		}
	}

	if link.Title == "" {
		link.Title = "(No Title)"
	}
	link.Rank = index + 1
	link.PostCount = stats.WeekCount.Posts
	link.RepostCount = stats.WeekCount.Reposts
	link.LikeCount = stats.WeekCount.Likes
	link.RecommendedPosts = recommendedPosts(app.Bluesky, stats.TopPosts())

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
