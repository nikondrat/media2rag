package rag

type RAGQuery struct {
	Query    string
	TopK     int
	Rerank   bool
	MaxTokens int
}

type RAGResponse struct {
	Answer  string
	Sources []Source
}

type Source struct {
	Index   int
	Title   string
	Type    string
	Source  string
	Content string
}

type QueryFormat string

const (
	FormatQuestion  QueryFormat = "question"
	FormatCommand   QueryFormat = "command"
	FormatStatement QueryFormat = "statement"
	FormatFragment  QueryFormat = "fragment"
)
