package model

type ExtractedContent struct {
	Title     string            `json:"title"`
	Author    string            `json:"author"`
	Source    string            `json:"source"`
	DocType   string            `json:"doc_type"`
	Content   string            `json:"content"`
	Language  string            `json:"language"`
	Sections  []Section         `json:"sections"`
	Images    []ExtractedImage  `json:"images"`
	Metadata  map[string]string `json:"metadata"`
	WordCount int               `json:"word_count"`
	CharCount int               `json:"char_count"`
}

type Section struct {
	Heading string `json:"heading"`
	Content string `json:"content"`
	Level   int    `json:"level"`
}

type ExtractedImage struct {
	Path    string `json:"path"`
	AltText string `json:"alt_text"`
	Caption string `json:"caption"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}
