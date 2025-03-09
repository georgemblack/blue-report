package bluesky

type Posts struct {
	Posts []Post `json:"posts"`
}

type Post struct {
	URI       string `json:"uri"`
	Author    Author `json:"author"`
	Record    Record `json:"record"`
	LikeCount int    `json:"likeCount"`
}

type Author struct {
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
}

type Record struct {
	CreatedAt string   `json:"createdAt"`
	Languages []string `json:"langs"`
	Text      string   `json:"text"`
}

func (p Post) IsEnglish() bool {
	for _, lang := range p.Record.Languages {
		if lang == "en" {
			return true
		}
	}
	return false
}
