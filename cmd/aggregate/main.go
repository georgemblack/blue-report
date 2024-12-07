package main

import (
	"log/slog"
	"os"

	"github.com/georgemblack/bluesky-links/pkg/app"
)

func main() {
	if os.Getenv("DEBUG") == "true" {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}
	err := app.Aggregate()
	if err != nil {
		slog.Error(err.Error())
	}
}
