// Package browserrun calls Cloudflare Browser Run, which loads a page in a
// real browser. It's used for pages that only show their details once
// JavaScript has run, or that turn away simple requests.
//
// Docs: https://developers.cloudflare.com/browser-run/
package browserrun

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Client calls Browser Run's REST API with an API token that has the
// "Browser Run - Edit" permission.
type Client struct {
	baseURL string
	token   string
	client  *http.Client
}

func New(accountID, token string) *Client {
	return &Client{
		baseURL: "https://api.cloudflare.com/client/v4/accounts/" + accountID + "/browser-run",
		token:   token,
		client:  &http.Client{Timeout: 45 * time.Second},
	}
}

// Element is one element found on a page.
type Element struct {
	Text       string
	Attributes map[string]string
}

// Result maps each selector to the elements it found, in page order.
type Result map[string][]Element

// Attr returns the first non-empty value of an attribute on the elements a
// selector found, or "".
func (r Result) Attr(selector, name string) string {
	for _, e := range r[selector] {
		if v := e.Attributes[name]; v != "" {
			return v
		}
	}
	return ""
}

// Text returns the text of the first element a selector found, or "".
func (r Result) Text(selector string) string {
	if es := r[selector]; len(es) > 0 {
		return es[0].Text
	}
	return ""
}

// Scrape loads a page and returns the elements matching each CSS selector.
// It also returns how much browser time the request used, which is what
// Cloudflare bills for.
func (c *Client) Scrape(ctx context.Context, pageURL string, selectors []string) (Result, time.Duration, error) {
	type element struct {
		Selector string `json:"selector"`
	}
	body := struct {
		URL      string    `json:"url"`
		Elements []element `json:"elements"`
		// Skip downloads that don't change what's on the page, to save
		// browser time.
		RejectResourceTypes []string `json:"rejectResourceTypes"`
		GotoOptions         struct {
			Timeout int `json:"timeout"` // milliseconds
		} `json:"gotoOptions"`
	}{URL: pageURL, RejectResourceTypes: []string{"image", "media", "font", "stylesheet"}}
	body.GotoOptions.Timeout = 20_000
	for _, s := range selectors {
		body.Elements = append(body.Elements, element{Selector: s})
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}

	var resp struct {
		Success bool `json:"success"`
		Result  []struct {
			Selector string `json:"selector"`
			Results  []struct {
				Text       string `json:"text"`
				Attributes []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"attributes"`
			} `json:"results"`
		} `json:"result"`
	}
	used, err := c.post(ctx, "/scrape", data, &resp)
	if err != nil {
		return nil, used, err
	}
	if !resp.Success {
		return nil, used, errors.New("browser run: scrape failed")
	}

	result := Result{}
	for _, r := range resp.Result {
		for _, found := range r.Results {
			e := Element{Text: found.Text, Attributes: map[string]string{}}
			for _, a := range found.Attributes {
				e.Attributes[a.Name] = a.Value
			}
			result[r.Selector] = append(result[r.Selector], e)
		}
	}
	return result, used, nil
}

// post sends a request, waiting and trying again (up to 3 times) when
// Cloudflare says we're sending too many. It returns the browser time used,
// from the X-Browser-Ms-Used header.
func (c *Client) post(ctx context.Context, path string, body []byte, v any) (time.Duration, error) {
	for range 3 {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return 0, err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.client.Do(req)
		if err != nil {
			return 0, fmt.Errorf("browser run: %w", err)
		}
		// A decimal number of milliseconds, such as "1253.9".
		ms, _ := strconv.ParseFloat(resp.Header.Get("X-Browser-Ms-Used"), 64)
		used := time.Duration(ms * float64(time.Millisecond))

		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			wait, err := strconv.Atoi(resp.Header.Get("Retry-After"))
			if err != nil || wait < 1 {
				wait = 1
			}
			select {
			case <-time.After(time.Duration(wait) * time.Second):
				continue
			case <-ctx.Done():
				return used, ctx.Err()
			}
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return used, fmt.Errorf("browser run: %s", resp.Status)
		}
		return used, json.NewDecoder(resp.Body).Decode(v)
	}
	return 0, errors.New("browser run: too many requests")
}
