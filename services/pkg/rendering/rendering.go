package rendering

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/georgemblack/blue-report/pkg/util"
)

const Retries = 3

func GetPageElements(token, accountID string, selectors []string, url string) (BrowserResponse, error) {
	reqBody := BrowserRequest{
		URL:      url,
		Elements: []BrowserRequestElements{},
	}
	for _, selector := range selectors {
		reqBody.Elements = append(reqBody.Elements, BrowserRequestElements{Selector: selector})
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return BrowserResponse{}, util.WrapErr("failed to marshal request body", err)
	}

	// Execute the request, and only retry if we get a 429
	for _ = range Retries {
		req, err := http.NewRequest("POST", fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/browser-rendering/scrape", accountID), bytes.NewBuffer(data))
		if err != nil {
			return BrowserResponse{}, util.WrapErr("failed to create request", err)
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{
			Timeout: 10 * time.Second,
		}
		resp, err := client.Do(req)
		if err != nil {
			return BrowserResponse{}, util.WrapErr("failed to send request", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			time.Sleep(1 * time.Second)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return BrowserResponse{}, fmt.Errorf("unexpected status code %s", resp.Status)
		}

		var browserRenderingResponse BrowserResponse
		if err := json.NewDecoder(resp.Body).Decode(&browserRenderingResponse); err != nil {
			return BrowserResponse{}, util.WrapErr("failed to decode response", err)
		}

		if !browserRenderingResponse.Success {
			return BrowserResponse{}, errors.New("request failed")
		}

		return browserRenderingResponse, nil
	}

	return BrowserResponse{}, errors.New("rate limited")
}
