package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/georgemblack/blue-report/pkg/app"
)

func main() {
	if os.Getenv("DEBUG") == "true" {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}
	application, err := app.NewApp()
	if err != nil {
		slog.Error(err.Error())
	}
	metadata := app.GetCardMetadata(application.Config, os.Args[1])
	slog.Info(fmt.Sprintf("title: %s, image url: %s", metadata.Title, metadata.ImageURL))
}
