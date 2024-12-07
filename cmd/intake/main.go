package main

import (
	"log/slog"

	"github.com/georgemblack/bluesky-links/pkg/app"
)

func main() {
	err := app.Intake()
	if err != nil {
		slog.Error(err.Error())
	}
}
