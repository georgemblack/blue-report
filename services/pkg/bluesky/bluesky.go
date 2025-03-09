package bluesky

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/georgemblack/blue-report/pkg/config"
)

type Bluesky struct {
	endpoint string
}

func New(cfg config.Config) Bluesky {
	return Bluesky{endpoint: cfg.BlueskyAPIEndpoint}
}

func (b Bluesky) GetPost(atURI string) (Post, error) {
	resp, err := http.Get(fmt.Sprintf("%s/xrpc/app.bsky.feed.getPosts?uris=%s", b.endpoint, atURI))
	if err != nil {
		return Post{}, err
	}
	defer resp.Body.Close()

	var posts Posts
	if err := json.NewDecoder(resp.Body).Decode(&posts); err != nil {
		return Post{}, err
	}

	if len(posts.Posts) == 0 {
		return Post{}, fmt.Errorf("post not found")
	}

	return posts.Posts[0], nil
}
