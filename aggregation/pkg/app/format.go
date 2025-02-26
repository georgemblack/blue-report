package app

import (
	"regexp"
	"strings"
)

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
