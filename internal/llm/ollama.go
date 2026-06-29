package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"media2rag/internal/model"
)

type OllamaClient struct {
	baseURL string
	model   string
	client  *http.Client
}

func NewOllamaClient(baseURL, model string, timeout time.Duration) *OllamaClient {
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &OllamaClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		client:  newConcurrentHTTPClient(timeout),
	}
}

type ollamaModel struct {
	Name string `json:"name"`
}

type ollamaTagsResponse struct {
	Models []ollamaModel `json:"models"`
}

func (c *OllamaClient) modelNotFoundError(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("model %q not found", c.model)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("model %q not found (and cannot reach ollama: %w)", c.model, err)
	}
	defer resp.Body.Close()

	var tags ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return fmt.Errorf("model %q not found", c.model)
	}

	if len(tags.Models) == 0 {
		return fmt.Errorf("model %q not found — no models installed in ollama", c.model)
	}

	names := make([]string, len(tags.Models))
	for i, m := range tags.Models {
		names[i] = m.Name
	}

	return fmt.Errorf("model %q not found. Available: %s. Set MEDIA2RAG_LLM_MODEL or fix config.yaml",
		c.model, strings.Join(names, ", "))
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  *ollamaOptions  `json:"options,omitempty"`
}

type ollamaOptions struct {
	NumPredict     int      `json:"num_predict,omitempty"`
	Stop           []string `json:"stop,omitempty"`
	FrequencyPenalty float64 `json:"frequency_penalty,omitempty"`
	PresencePenalty  float64 `json:"presence_penalty,omitempty"`
}

type ollamaMessage struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
}

type ollamaChatResponse struct {
	Model            string        `json:"model"`
	CreatedAt        string        `json:"created_at"`
	Message          ollamaMessage `json:"message"`
	Done             bool          `json:"done"`
	PromptEvalCount  int           `json:"prompt_eval_count"`
	EvalCount        int           `json:"eval_count"`
}

type ollamaStreamResponse struct {
	Model            string `json:"model"`
	CreatedAt        string `json:"created_at"`
	Content          string `json:"content"`
	Done             bool   `json:"done"`
	PromptEvalCount  int    `json:"prompt_eval_count"`
	EvalCount        int    `json:"eval_count"`
}

type ollamaEmbedRequest struct {
	Model  string      `json:"model"`
	Input  interface{} `json:"input"`
}

type ollamaEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

func (c *OllamaClient) Chat(ctx context.Context, req model.ChatRequest) (*model.ChatResponse, error) {
	messages := make([]ollamaMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = ollamaMessage{Role: m.Role, Content: m.Content}
	}

	if len(req.Images) > 0 {
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Role == "user" {
				messages[i].Images = req.Images
				break
			}
		}
	}

	body := ollamaChatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   false,
	}

	if req.MaxTokens != nil || len(req.Stop) > 0 || req.FrequencyPenalty != nil || req.PresencePenalty != nil {
		opts := &ollamaOptions{}
		if req.MaxTokens != nil {
			opts.NumPredict = *req.MaxTokens
		}
		if len(req.Stop) > 0 {
			opts.Stop = req.Stop
		}
		if req.FrequencyPenalty != nil {
			opts.FrequencyPenalty = *req.FrequencyPenalty
		}
		if req.PresencePenalty != nil {
			opts.PresencePenalty = *req.PresencePenalty
		}
		body.Options = opts
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", model.ErrLLMUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, c.modelNotFoundError(ctx)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: ollama returned status %d", model.ErrLLMUnavailable, resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if len(bodyBytes) == 0 {
		return nil, fmt.Errorf("ollama returned empty response body")
	}

	var ollamaResp ollamaChatResponse
	if err := json.Unmarshal(bodyBytes, &ollamaResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	usage := &model.Usage{
		PromptTokens:     ollamaResp.PromptEvalCount,
		CompletionTokens: ollamaResp.EvalCount,
		TotalTokens:      ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
	}
	if usage.TotalTokens == 0 {
		usage = nil
	}

	return &model.ChatResponse{
		Model: ollamaResp.Model,
		Message: model.Message{
			Role:    ollamaResp.Message.Role,
			Content: ollamaResp.Message.Content,
		},
		Done:  ollamaResp.Done,
		Usage: usage,
	}, nil
}

func (c *OllamaClient) StreamChat(ctx context.Context, req model.ChatRequest) (<-chan model.StreamDelta, error) {
	messages := make([]ollamaMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = ollamaMessage{Role: m.Role, Content: m.Content}
	}

	body := ollamaChatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   true,
	}

	if req.MaxTokens != nil || len(req.Stop) > 0 || req.FrequencyPenalty != nil || req.PresencePenalty != nil {
		opts := &ollamaOptions{}
		if req.MaxTokens != nil {
			opts.NumPredict = *req.MaxTokens
		}
		if len(req.Stop) > 0 {
			opts.Stop = req.Stop
		}
		if req.FrequencyPenalty != nil {
			opts.FrequencyPenalty = *req.FrequencyPenalty
		}
		if req.PresencePenalty != nil {
			opts.PresencePenalty = *req.PresencePenalty
		}
		body.Options = opts
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", model.ErrLLMUnavailable, err)
	}

	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, c.modelNotFoundError(ctx)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%w: ollama returned status %d", model.ErrLLMUnavailable, resp.StatusCode)
	}

	ch := make(chan model.StreamDelta)

	go func() {
		defer resp.Body.Close()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var streamResp ollamaStreamResponse
			if err := json.Unmarshal(line, &streamResp); err != nil {
				continue
			}

			ch <- model.StreamDelta{
				Content:          streamResp.Content,
				Done:             streamResp.Done,
				PromptEvalCount:  streamResp.PromptEvalCount,
				EvalCount:        streamResp.EvalCount,
			}

			if streamResp.Done {
				return
			}
		}
	}()

	return ch, nil
}

func (c *OllamaClient) ChatAndParse(ctx context.Context, req model.ChatRequest) ([]model.TypedBlock, error) {
	resp, err := c.Chat(ctx, req)
	if err != nil {
		return nil, err
	}
	return ParseOutput(resp.Message.Content)
}

func (c *OllamaClient) StreamAndParse(ctx context.Context, req model.ChatRequest) (<-chan model.StreamDelta, chan []model.TypedBlock, error) {
	deltaCh, err := c.StreamChat(ctx, req)
	if err != nil {
		return nil, nil, err
	}

	resultCh := make(chan []model.TypedBlock, 1)
	go func() {
		defer close(resultCh)
		var sb strings.Builder
		for delta := range deltaCh {
			sb.WriteString(delta.Content)
		}
		blocks, _ := ParseOutput(sb.String())
		resultCh <- blocks
	}()

	return deltaCh, resultCh, nil
}

func (c *OllamaClient) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := c.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return embeddings[0], nil
}

func (c *OllamaClient) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	body := ollamaEmbedRequest{
		Model: c.model,
		Input: texts,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/embed", &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", model.ErrLLMUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, c.modelNotFoundError(ctx)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: ollama returned status %d", model.ErrLLMUnavailable, resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var batchResp struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.Unmarshal(bodyBytes, &batchResp); err != nil {
		return nil, fmt.Errorf("decode batch response: %w", err)
	}

	return batchResp.Embeddings, nil
}
