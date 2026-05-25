package llm

import (
	"context"
	"errors"

	"media2rag/internal/model"
)

func NewClient(ctx context.Context, backend, ollamaURL, model, openRouterURL, openRouterKey string) (LLMClient, error) {
	primary, err := primaryClient(ctx, backend, ollamaURL, model, openRouterURL, openRouterKey)
	if err != nil {
		return nil, err
	}

	if openRouterKey != "" {
		fallback := NewOpenRouterClient(openRouterURL, openRouterKey, model)
		return &fallbackClient{primary: primary, fallback: fallback}, nil
	}

	return primary, nil
}

func primaryClient(ctx context.Context, backend, ollamaURL, model, openRouterURL, openRouterKey string) (LLMClient, error) {
	if backend == "openrouter" {
		return NewOpenRouterClient(openRouterURL, openRouterKey, model), nil
	}
	return NewOllamaClient(ollamaURL, model), nil
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
