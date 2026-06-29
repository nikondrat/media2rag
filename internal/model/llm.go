package model

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatRequest struct {
	Model            string    `json:"model"`
	Messages         []Message `json:"messages"`
	Stream           bool      `json:"stream"`
	Images           []string  `json:"images,omitempty"`
	MaxTokens        *int      `json:"max_tokens,omitempty"`
	Stop             []string  `json:"stop,omitempty"`
	FrequencyPenalty *float64  `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64  `json:"presence_penalty,omitempty"`
}

type ChatResponse struct {
	Model   string  `json:"model"`
	Message Message `json:"message"`
	Done    bool    `json:"done"`
	Usage   *Usage  `json:"usage,omitempty"`
}

type StreamDelta struct {
	Model            string `json:"model"`
	Content          string `json:"content"`
	Done             bool   `json:"done"`
	PromptEvalCount  int    `json:"prompt_eval_count,omitempty"`
	EvalCount        int    `json:"eval_count,omitempty"`
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
