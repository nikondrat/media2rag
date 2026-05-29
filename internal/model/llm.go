package model

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
	Images   []string  `json:"images,omitempty"`
}

type ChatResponse struct {
	Model   string  `json:"model"`
	Message Message `json:"message"`
	Done    bool    `json:"done"`
}

type StreamDelta struct {
	Model   string `json:"model"`
	Content string `json:"content"`
	Done    bool   `json:"done"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type TypedBlock struct {
	Type    string            `json:"type"`
	Params  map[string]string `json:"params,omitempty"`
	Content string            `json:"content"`
}
