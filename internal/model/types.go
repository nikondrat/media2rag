package model

import "errors"

type ExtractedContent struct {
	Title      string            `json:"title"`
	Author     string            `json:"author"`
	Source     string            `json:"source"`
	DocType    string            `json:"doc_type"`
	Content    string            `json:"content"`
	Language   string            `json:"language"`
	Sections   []Section         `json:"sections"`
	Images     []ExtractedImage  `json:"images"`
	Metadata   map[string]string `json:"metadata"`
	WordCount  int               `json:"word_count"`
	CharCount  int               `json:"char_count"`
}

type Section struct {
	Heading string `json:"heading"`
	Content string `json:"content"`
	Level   int    `json:"level"`
}

type ExtractedImage struct {
	Path        string `json:"path"`
	AltText     string `json:"alt_text"`
	Caption     string `json:"caption"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

type RAGDocument struct {
	Markdown string           `json:"markdown"`
	Metadata DocumentMetadata `json:"metadata"`
}

type DocumentMetadata struct {
	Title        string   `json:"title"`
	Author       string   `json:"author"`
	Source       string   `json:"source"`
	DocType      string   `json:"doc_type"`
	Language     string   `json:"language"`
	Domains      []string `json:"domains"`
	CoreThesis   string   `json:"core_thesis"`
	MentalModels []string `json:"mental_models"`
	Claims       []Claim  `json:"claims"`
	Takeaways    []string `json:"takeaways"`
	KeyTerms     []KeyTerm `json:"key_terms"`
	Summary      string   `json:"summary"`
	KeyInsights  []string `json:"key_insights"`
	WordCount    int      `json:"word_count"`
	Topics       []string `json:"topics"`
}

type Claim struct {
	Statement string  `json:"statement"`
	Confidence float64 `json:"confidence"`
	Source     string `json:"source"`
}

type KeyTerm struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
	Images   []string  `json:"images,omitempty"`
}

type ChatResponse struct {
	Model     string  `json:"model"`
	Message   Message `json:"message"`
	Done      bool    `json:"done"`
}

type StreamDelta struct {
	Model     string `json:"model"`
	Content   string `json:"content"`
	Done      bool   `json:"done"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Event struct {
	Type     string      `json:"type"`
	Data     interface{} `json:"data,omitempty"`
	Progress float64     `json:"progress,omitempty"`
	Error    string      `json:"error,omitempty"`
}

type MemoryEntry struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Content   string `json:"content"`
	Category  string `json:"category"`
	CreatedAt int64  `json:"created_at"`
}

var (
	ErrExtractionFailed = errors.New("extraction failed")
	ErrLLMUnavailable   = errors.New("LLM unavailable")
	ErrConfigInvalid    = errors.New("invalid configuration")
	ErrFileNotFound     = errors.New("file not found")
	ErrInvalidInput     = errors.New("invalid input")
)
