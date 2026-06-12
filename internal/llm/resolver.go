package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"media2rag/internal/model"
)

func NewClient(ctx context.Context, backend, ollamaURL, model, openRouterURL, openRouterKey string, lmStudioURL string, timeout time.Duration) (LLMClient, error) {
	primary, err := primaryClient(ctx, backend, ollamaURL, model, openRouterURL, openRouterKey, lmStudioURL, timeout)
	if err != nil {
		return nil, err
	}

	if openRouterKey != "" {
		fallback := NewOpenRouterClient(openRouterURL, openRouterKey, model, timeout)
		return &fallbackClient{primary: primary, fallback: fallback}, nil
	}

	return primary, nil
}

func primaryClient(ctx context.Context, backend, ollamaURL, model, openRouterURL, openRouterKey string, lmStudioURL string, timeout time.Duration) (LLMClient, error) {
	switch backend {
	case "openrouter":
		return NewOpenRouterClient(openRouterURL, openRouterKey, model, timeout), nil
	case "lmstudio":
		return NewOpenRouterClient(lmStudioURL, "", model, timeout), nil
	}
	return NewOllamaClient(ollamaURL, model, timeout), nil
}

type fallbackClient struct {
	primary  LLMClient
	fallback LLMClient
}

func (f *fallbackClient) Chat(ctx context.Context, req model.ChatRequest) (*model.ChatResponse, error) {
	resp, err := f.primary.Chat(ctx, req)
	if err != nil {
		if errors.Is(err, model.ErrLLMUnavailable) {
			return f.fallback.Chat(ctx, req)
		}
		return nil, err
	}
	return resp, nil
}

func (f *fallbackClient) StreamChat(ctx context.Context, req model.ChatRequest) (<-chan model.StreamDelta, error) {
	resp, err := f.primary.StreamChat(ctx, req)
	if err != nil {
		if errors.Is(err, model.ErrLLMUnavailable) {
			return f.fallback.StreamChat(ctx, req)
		}
		return nil, err
	}
	return resp, nil
}

func (f *fallbackClient) ChatAndParse(ctx context.Context, req model.ChatRequest) ([]model.TypedBlock, error) {
	resp, err := f.primary.ChatAndParse(ctx, req)
	if err != nil {
		if errors.Is(err, model.ErrLLMUnavailable) {
			return f.fallback.ChatAndParse(ctx, req)
		}
		return nil, err
	}
	return resp, nil
}

func (f *fallbackClient) StreamAndParse(ctx context.Context, req model.ChatRequest) (<-chan model.StreamDelta, chan []model.TypedBlock, error) {
	resp, result, err := f.primary.StreamAndParse(ctx, req)
	if err != nil {
		if errors.Is(err, model.ErrLLMUnavailable) {
			return f.fallback.StreamAndParse(ctx, req)
		}
		return nil, nil, err
	}
	return resp, result, nil
}

func (f *fallbackClient) Embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := f.primary.Embed(ctx, text)
	if err != nil {
		if errors.Is(err, model.ErrLLMUnavailable) {
			return f.fallback.Embed(ctx, text)
		}
		return nil, err
	}
	return resp, nil
}

func (f *fallbackClient) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	resp, err := f.primary.EmbedBatch(ctx, texts)
	if err != nil {
		if errors.Is(err, model.ErrLLMUnavailable) {
			return f.fallback.EmbedBatch(ctx, texts)
		}
		return nil, err
	}
	return resp, nil
}

func NewClientFromChain(ctx context.Context, models []string, backend, ollamaURL, openRouterURL, openRouterKey string, timeout time.Duration) (LLMClient, error) {
	if len(models) == 0 {
		return nil, fmt.Errorf("empty model chain")
	}

	primary, err := newClientForModel(ctx, models[0], backend, ollamaURL, openRouterURL, openRouterKey, timeout)
	if err != nil {
		return nil, err
	}

	if len(models) == 1 {
		return primary, nil
	}

	client := &chainClient{clients: make([]LLMClient, len(models))}
	client.clients[0] = primary

	for i := 1; i < len(models); i++ {
		c, err := newClientForModel(ctx, models[i], backend, ollamaURL, openRouterURL, openRouterKey, timeout)
		if err != nil {
			return nil, err
		}
		client.clients[i] = c
	}

	return client, nil
}

func newClientForModel(ctx context.Context, model, backend, ollamaURL, openRouterURL, openRouterKey string, timeout time.Duration) (LLMClient, error) {
	if openRouterKey != "" {
		if strings.Contains(model, "openrouter") || strings.Contains(model, "/") {
			return NewOpenRouterClient(openRouterURL, openRouterKey, model, timeout), nil
		}
	}
	return NewOllamaClient(ollamaURL, model, timeout), nil
}

type chainClient struct {
	clients []LLMClient
}

func (c *chainClient) tryChain(ctx context.Context, fn func(LLMClient) error) error {
	var errs []error
	for _, client := range c.clients {
		err := fn(client)
		if err == nil {
			return nil
		}
		if errors.Is(err, model.ErrLLMUnavailable) || isRetryable(err) {
			errs = append(errs, err)
			continue
		}
		return err
	}
	return fmt.Errorf("all models in chain failed: %w", errors.Join(errs...))
}

func (c *chainClient) Chat(ctx context.Context, req model.ChatRequest) (*model.ChatResponse, error) {
	var resp *model.ChatResponse
	err := c.tryChain(ctx, func(client LLMClient) error {
		var err error
		resp, err = client.Chat(ctx, req)
		return err
	})
	return resp, err
}

func (c *chainClient) StreamChat(ctx context.Context, req model.ChatRequest) (<-chan model.StreamDelta, error) {
	return c.clients[0].StreamChat(ctx, req)
}

func (c *chainClient) ChatAndParse(ctx context.Context, req model.ChatRequest) ([]model.TypedBlock, error) {
	var blocks []model.TypedBlock
	err := c.tryChain(ctx, func(client LLMClient) error {
		var err error
		blocks, err = client.ChatAndParse(ctx, req)
		return err
	})
	return blocks, err
}

func (c *chainClient) StreamAndParse(ctx context.Context, req model.ChatRequest) (<-chan model.StreamDelta, chan []model.TypedBlock, error) {
	return c.clients[0].StreamAndParse(ctx, req)
}

func (c *chainClient) Embed(ctx context.Context, text string) ([]float32, error) {
	return c.clients[0].Embed(ctx, text)
}

func (c *chainClient) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	return c.clients[0].EmbedBatch(ctx, texts)
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return containsAny(msg, "rate limit", "timeout", "5xx", "503", "502", "429", "too many requests", "unavailable")
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func init() {
}
