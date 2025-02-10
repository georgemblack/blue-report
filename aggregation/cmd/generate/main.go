package main

import (
	"log/slog"
	"os"

	"github.com/georgemblack/blue-report/pkg/app"
)

func main() {
	if os.Getenv("DEBUG") == "true" {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}
	snapshot, err := app.Generate()
	if err != nil {
		slog.Error(err.Error())
	}
	err = app.Publish(snapshot)
	if err != nil {
		slog.Error(err.Error())
	}
}
