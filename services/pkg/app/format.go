package app

import (
	"regexp"
	"strings"
)

var truncatedURLPattern = regexp.MustCompile(`(www\.)?[\w.-]+\.[a-z]{2,}(/[^\s]*)?\w*\.{3}`)

var badPrefixes = []string{
	"BREAKING:",
	"Breaking:",
	"BREAKING NEWS:",
	"Breaking News:",
	"NEW:",
	"New:",
	"EXCLUSIVE:",
	"Exclusive:",
	"🔴",
	"💥",
}

func formatTitle(title string) string {
	// Remove any siren emojis, they are annoying
	title = strings.ReplaceAll(title, "🚨", "")

	// Remove any sensationalist prefixes
	for _, prefix := range badPrefixes {
		title = strings.TrimPrefix(title, prefix)
	}

	return strings.TrimSpace(title)
}

func formatPost(text string) string {
	// Clean up any newlines or extra whitespace
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.ReplaceAll(text, "\t", " ")

	// Remove any siren emojis, they are annoying
	text = strings.ReplaceAll(text, "🚨", "")

	// Remove any sensationalist prefixes
	for _, prefix := range badPrefixes {
		text = strings.TrimPrefix(text, prefix)
	}

	// Collapse all whitespace into a single space
	text = strings.Join(strings.Fields(text), " ")

	// Remove URLs from the post text, as it is redundant.
	// The Bluesky post editor frequently truncates URL, so they appear as the following:
	//  - 'www.comicsands.com/crockett-bro...'
	// 	- 'apnews.com/article/trum...
	// 	- 'www.democracydocket.com/opinion/my-o...'
	// Use regex to find URLs that match this pattern and remove them.
	cleaned := truncatedURLPattern.ReplaceAllString(text, "")
	trimmed := strings.TrimSpace(cleaned)

	return trimmed
}
