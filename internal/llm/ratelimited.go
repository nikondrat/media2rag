package llm

import (
	"context"
	"strings"

	"media2rag/internal/model"
)

type RateLimitedClient struct {
	inner LLMClient
	sem   chan struct{}
}

func NewRateLimitedClient(inner LLMClient, maxConcurrent int) *RateLimitedClient {
	if maxConcurrent <= 0 {
		maxConcurrent = 50
	}
	return &RateLimitedClient{
		inner: inner,
		sem:   make(chan struct{}, maxConcurrent),
	}
}

func (c *RateLimitedClient) acquire(ctx context.Context) error {
	select {
	case c.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *RateLimitedClient) Chat(ctx context.Context, req model.ChatRequest) (*model.ChatResponse, error) {
	if err := c.acquire(ctx); err != nil {
		return nil, err
	}
	defer func() { <-c.sem }()
	return c.inner.Chat(ctx, req)
}

func (c *RateLimitedClient) StreamChat(ctx context.Context, req model.ChatRequest) (<-chan model.StreamDelta, error) {
	if err := c.acquire(ctx); err != nil {
		return nil, err
	}
	ch, err := c.inner.StreamChat(ctx, req)
	if err != nil {
		<-c.sem
		return nil, err
	}
	wrapped := make(chan model.StreamDelta)
	go func() {
		defer close(wrapped)
		for delta := range ch {
			wrapped <- delta
			if delta.Done {
				<-c.sem
			}
		}
	}()
	return wrapped, nil
}

func (c *RateLimitedClient) Embed(ctx context.Context, text string) ([]float32, error) {
	if err := c.acquire(ctx); err != nil {
		return nil, err
	}
	defer func() { <-c.sem }()
	return c.inner.Embed(ctx, text)
}

func (c *RateLimitedClient) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if err := c.acquire(ctx); err != nil {
		return nil, err
	}
	defer func() { <-c.sem }()
	return c.inner.EmbedBatch(ctx, texts)
}

func (c *RateLimitedClient) ChatAndParse(ctx context.Context, req model.ChatRequest) ([]model.TypedBlock, error) {
	resp, err := c.Chat(ctx, req)
	if err != nil {
		return nil, err
	}
	return ParseOutput(resp.Message.Content)
}

func (c *RateLimitedClient) StreamAndParse(ctx context.Context, req model.ChatRequest) (<-chan model.StreamDelta, chan []model.TypedBlock, error) {
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
