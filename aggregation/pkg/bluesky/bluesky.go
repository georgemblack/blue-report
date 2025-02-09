package bluesky

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const publicEndpoint = "https://public.api.bsky.app"

type Bluesky struct {
	endpoint string
}

func New() Bluesky {
	return Bluesky{endpoint: publicEndpoint}
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
