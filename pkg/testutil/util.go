package testutil

import (
	"embed"
	"encoding/json"
	"fmt"

	"github.com/georgemblack/blue-report/pkg/app"
)

//go:embed test-data/*
var files embed.FS

func GetStreamEvent(name string) (app.StreamEvent, error) {
	bytes, err := files.ReadFile(fmt.Sprintf("test-data/%s", name))
	if err != nil {
		return app.StreamEvent{}, err
	}

	var event app.StreamEvent
	err = json.Unmarshal(bytes, &event)
	if err != nil {
		return app.StreamEvent{}, err
	}

	return event, nil
}
