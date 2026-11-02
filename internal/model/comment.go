package model

// Comment is one viewer danmaku line entering the pipeline.
type Comment struct {
	User   string `json:"user"`
	Text   string `json:"text"`
	Source string `json:"source"`
}
