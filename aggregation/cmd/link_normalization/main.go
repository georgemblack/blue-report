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
	err := app.NormalizeLinks()
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
