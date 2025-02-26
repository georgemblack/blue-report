package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/georgemblack/blue-report/pkg/util"
)

type CardyB struct {
	Title string `json:"title"`
	Image string `json:"image"`
}

// Fetch metadata for a given URL via Bluesky's CardyB service.
func cardyB(url string) (CardyB, error) {
	resp, err := http.Get("https://cardyb.bsky.app/v1/extract?url=" + url)
	if err != nil {
		return CardyB{}, util.WrapErr("failed to get title from cardyb", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return CardyB{}, errors.New("failed to get title from cardyb: status code " + resp.Status)
	}

	var cardyB CardyB
	if err := json.NewDecoder(resp.Body).Decode(&cardyB); err != nil {
		return CardyB{}, util.WrapErr("failed to decode cardyb response", err)
	}

	time.Sleep(1 * time.Second)

	return cardyB, nil
}
