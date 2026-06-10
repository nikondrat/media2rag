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

type OpenRouterClient struct {
	baseURL  string
	apiKey   string
	model    string
	client   *http.Client
	maxRetry int
}

func NewOpenRouterClient(baseURL, apiKey, model string, timeout time.Duration) *OpenRouterClient {
	if timeout <= 0 {
		timeout = 300 * time.Second
	}
	return &OpenRouterClient{
		baseURL:  strings.TrimRight(baseURL, "/"),
		apiKey:   apiKey,
		model:    model,
		client:   newConcurrentHTTPClient(timeout),
		maxRetry: 3,
	}
}

type openRouterMessage struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

type openRouterRequest struct {
	Model    string              `json:"model"`
	Messages []openRouterMessage `json:"messages"`
	Stream   bool                `json:"stream"`
}

type openRouterChoice struct {
	Index        int               `json:"index"`
	Message      openRouterMessage `json:"message"`
	FinishReason string            `json:"finish_reason"`
}

type openRouterUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openRouterResponse struct {
	ID      string            `json:"id"`
	Model   string            `json:"model"`
	Choices []openRouterChoice `json:"choices"`
	Usage   *openRouterUsage  `json:"usage,omitempty"`
}

type openRouterStreamChoice struct {
	Delta struct {
		Content string `json:"content"`
	} `json:"delta"`
	FinishReason *string `json:"finish_reason"`
}

type openRouterStreamResponse struct {
	Choices []openRouterStreamChoice `json:"choices"`
}

func isRetryableStatus(status int) bool {
	return status == http.StatusTooManyRequests ||
		status == http.StatusServiceUnavailable ||
		status == http.StatusBadGateway ||
		status == http.StatusGatewayTimeout ||
		status == http.StatusInternalServerError
}

func (c *OpenRouterClient) doWithRetry(ctx context.Context, reqBody openRouterRequest) (*openRouterResponse, error) {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetry; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * 2 * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(reqBody); err != nil {
			return nil, fmt.Errorf("encode request: %w", err)
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", &buf)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

		resp, err := c.client.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("openrouter request failed: %w", err)
			continue
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read response body: %w", err)
			continue
		}

		if resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("openrouter: authentication failed (401)")
		}

		if isRetryableStatus(resp.StatusCode) {
			lastErr = fmt.Errorf("openrouter: returned status %d (attempt %d/%d)", resp.StatusCode, attempt+1, c.maxRetry+1)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("openrouter: returned status %d: %s", resp.StatusCode, string(bodyBytes[:min(len(bodyBytes), 500)]))
		}

		var openAIResp openRouterResponse
		if err := json.Unmarshal(bodyBytes, &openAIResp); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}

		if len(openAIResp.Choices) == 0 {
			return nil, fmt.Errorf("openrouter: no choices returned")
		}

		return &openAIResp, nil
	}

	return nil, fmt.Errorf("openrouter: all %d attempts failed: %w", c.maxRetry+1, lastErr)
}

func (c *OpenRouterClient) Chat(ctx context.Context, req model.ChatRequest) (*model.ChatResponse, error) {
	messages := make([]openRouterMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = openRouterMessage{Role: m.Role, Content: m.Content}
	}

	body := openRouterRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   false,
	}

	openAIResp, err := c.doWithRetry(ctx, body)
	if err != nil {
		return nil, err
	}

	content := openAIResp.Choices[0].Message.Content
	if content == "" {
		content = openAIResp.Choices[0].Message.ReasoningContent
	}

	var usage *model.Usage
	if openAIResp.Usage != nil {
		usage = &model.Usage{
			PromptTokens:     openAIResp.Usage.PromptTokens,
			CompletionTokens: openAIResp.Usage.CompletionTokens,
			TotalTokens:      openAIResp.Usage.TotalTokens,
		}
	}

	return &model.ChatResponse{
		Model: openAIResp.Model,
		Message: model.Message{
			Role:    openAIResp.Choices[0].Message.Role,
			Content: content,
		},
		Done:  true,
		Usage: usage,
	}, nil
}

func (c *OpenRouterClient) StreamChat(ctx context.Context, req model.ChatRequest) (<-chan model.StreamDelta, error) {
	messages := make([]openRouterMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = openRouterMessage{Role: m.Role, Content: m.Content}
	}

	body := openRouterRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   true,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openrouter request failed: %w", err)
	}

	ch := make(chan model.StreamDelta)

	go func() {
		defer resp.Body.Close()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				ch <- model.StreamDelta{Done: true}
				return
			}

			var streamResp openRouterStreamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				continue
			}

			for _, choice := range streamResp.Choices {
				ch <- model.StreamDelta{
					Content: choice.Delta.Content,
					Done:    choice.FinishReason != nil,
				}
			}
		}
	}()

	return ch, nil
}

func (c *OpenRouterClient) ChatAndParse(ctx context.Context, req model.ChatRequest) ([]model.TypedBlock, error) {
	resp, err := c.Chat(ctx, req)
	if err != nil {
		return nil, err
	}
	return ParseOutput(resp.Message.Content)
}

func (c *OpenRouterClient) StreamAndParse(ctx context.Context, req model.ChatRequest) (<-chan model.StreamDelta, chan []model.TypedBlock, error) {
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

func (c *OpenRouterClient) Embed(ctx context.Context, text string) ([]float32, error) {
	// Try /v1/embeddings endpoint (works with LM Studio, Ollama-compatible servers)
	type embedInput struct {
		Model string   `json:"model"`
		Input []string `json:"input"`
	}
	type embedData struct {
		Object    string    `json:"object"`
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	}
	type embedResponse struct {
		Data []embedData `json:"data"`
	}

	reqBody := embedInput{
		Model: c.model,
		Input: []string{text},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(reqBody); err != nil {
		return nil, fmt.Errorf("encode embed request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/embeddings", &buf)
	if err != nil {
		return nil, fmt.Errorf("create embed request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("embed request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read embed response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embed error (%d): %s", resp.StatusCode, string(bodyBytes[:min(len(bodyBytes), 200)]))
	}

	var embedResp embedResponse
	if err := json.Unmarshal(bodyBytes, &embedResp); err != nil {
		return nil, fmt.Errorf("decode embed response: %w", err)
	}

	if len(embedResp.Data) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return embedResp.Data[0].Embedding, nil
}

func (c *OpenRouterClient) Model() string {
	return c.model
}