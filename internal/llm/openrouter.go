package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"media2rag/internal/model"
)

type OpenRouterClient struct {
	baseURL  string
	apiKey   string
	model    string
	client   *http.Client
}

func NewOpenRouterClient(baseURL, apiKey, model string) *OpenRouterClient {
	return &OpenRouterClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		client:  &http.Client{},
	}
}

type openRouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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

type openRouterResponse struct {
	ID      string            `json:"id"`
	Model   string            `json:"model"`
	Choices []openRouterChoice `json:"choices"`
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
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("openrouter: authentication failed (401)")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openrouter: returned status %d", resp.StatusCode)
	}

	var openAIResp openRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("openrouter: no choices returned")
	}

	return &model.ChatResponse{
		Model: openAIResp.Model,
		Message: model.Message{
			Role:    openAIResp.Choices[0].Message.Role,
			Content: openAIResp.Choices[0].Message.Content,
		},
		Done: true,
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

func (c *OpenRouterClient) Embed(ctx context.Context, text string) ([]float32, error) {
	return nil, fmt.Errorf("embed not supported by OpenRouter")
}
