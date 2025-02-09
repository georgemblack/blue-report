package bluesky

type Posts struct {
	Posts []Post `json:"posts"`
}

type Post struct {
	URI    string `json:"uri"`
	Author Author `json:"author"`
	Record Record `json:"record"`
}

type Author struct {
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
}

type Record struct {
	CreatedAt string `json:"createdAt"`
	Text      string `json:"text"`
}
