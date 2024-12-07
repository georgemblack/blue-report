package main

import (
	"log/slog"

	"github.com/georgemblack/bluesky-links/pkg/app"
)

func main() {
	err := app.Aggregate()
	if err != nil {
		slog.Error(err.Error())
	}
}
