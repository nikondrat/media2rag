package llm

import (
	"context"
	"time"

	"media2rag/internal/model"
)

type PricingStore interface {
	CalculateCost(model string, promptTokens, completionTokens int) float64
}

type InstrumentedClient struct {
	inner    LLMClient
	pricing  PricingStore
	recorder model.TelemetryRecorder
}

func NewInstrumentedClient(inner LLMClient, pricing PricingStore, recorder model.TelemetryRecorder) *InstrumentedClient {
	if pricing == nil {
		pricing = &defaultPricingStore{}
	}
	return &InstrumentedClient{
		inner:    inner,
		pricing:  pricing,
		recorder: recorder,
	}
}

type defaultPricingStore struct{}

func (d *defaultPricingStore) CalculateCost(model string, promptTokens, completionTokens int) float64 {
	return CalculateCost(model, promptTokens, completionTokens)
}

func (c *InstrumentedClient) Chat(ctx context.Context, req model.ChatRequest) (*model.ChatResponse, error) {
	start := time.Now()
	resp, err := c.inner.Chat(ctx, req)
	latencyMs := time.Since(start).Milliseconds()

	t := model.LLMTelemetry{
		Source:          sourceFromCtx(ctx),
		Stage:           stageFromCtx(ctx),
		ChunkIndex:      chunkFromCtx(ctx),
		RetryAttempt:    retryFromCtx(ctx),
		Timestamp:       time.Now(),
		LatencyMs:       latencyMs,
		PromptChars:     promptChars(req.Messages),
	}

	t.Model = c.Model()

	if err != nil {
		t.Success = false
		t.Error = err.Error()
		c.recorder.Record(t)
		return nil, err
	}

	t.Success = true
	t.Model = resp.Model

	if resp.Usage != nil {
		t.PromptTokens = resp.Usage.PromptTokens
		t.CompletionTokens = resp.Usage.CompletionTokens
	}

	t.CompletionChars = len(resp.Message.Content)
	t.Cost = c.pricing.CalculateCost(resp.Model, t.PromptTokens, t.CompletionTokens)

	c.recorder.Record(t)
	return resp, nil
}

func (c *InstrumentedClient) StreamChat(ctx context.Context, req model.ChatRequest) (<-chan model.StreamDelta, error) {
	innerCh, err := c.inner.StreamChat(ctx, req)
	if err != nil {
		t := model.LLMTelemetry{
			Source:       sourceFromCtx(ctx),
			Stage:        stageFromCtx(ctx),
			ChunkIndex:   chunkFromCtx(ctx),
			RetryAttempt: retryFromCtx(ctx),
			Timestamp:    time.Now(),
			Success:      false,
			Error:        err.Error(),
			Model:        c.Model(),
			PromptChars:  promptChars(req.Messages),
		}
		c.recorder.Record(t)
		return nil, err
	}

	modelName := c.Model()
	outCh := make(chan model.StreamDelta)
	start := time.Now()

	go func() {
		defer close(outCh)
		var accumulated string
		var finalUsage *model.Usage

		for delta := range innerCh {
			accumulated += delta.Content
			if delta.Model != "" {
				modelName = delta.Model
			}
			outCh <- delta

			if delta.Done {
				finalUsage = c.extractStreamUsage(delta)
			}
		}

		latencyMs := time.Since(start).Milliseconds()

		t := model.LLMTelemetry{
			Source:           sourceFromCtx(ctx),
			Stage:            stageFromCtx(ctx),
			ChunkIndex:       chunkFromCtx(ctx),
			RetryAttempt:     retryFromCtx(ctx),
			Timestamp:        time.Now(),
			LatencyMs:        latencyMs,
			Success:          true,
			Model:            modelName,
			PromptChars:      promptChars(req.Messages),
			CompletionChars:  len(accumulated),
		}

		if finalUsage != nil {
			t.PromptTokens = finalUsage.PromptTokens
			t.CompletionTokens = finalUsage.CompletionTokens
		}

		t.Cost = c.pricing.CalculateCost(t.Model, t.PromptTokens, t.CompletionTokens)
		c.recorder.Record(t)
	}()

	return outCh, nil
}

func (c *InstrumentedClient) extractStreamUsage(delta model.StreamDelta) *model.Usage {
	if delta.PromptEvalCount == 0 && delta.EvalCount == 0 {
		return nil
	}
	return &model.Usage{
		PromptTokens:     delta.PromptEvalCount,
		CompletionTokens: delta.EvalCount,
		TotalTokens:      delta.PromptEvalCount + delta.EvalCount,
	}
}

func (c *InstrumentedClient) Embed(ctx context.Context, text string) ([]float32, error) {
	return c.inner.Embed(ctx, text)
}

func (c *InstrumentedClient) ChatAndParse(ctx context.Context, req model.ChatRequest) ([]model.TypedBlock, error) {
	return c.inner.ChatAndParse(ctx, req)
}

func (c *InstrumentedClient) StreamAndParse(ctx context.Context, req model.ChatRequest) (<-chan model.StreamDelta, chan []model.TypedBlock, error) {
	return c.inner.StreamAndParse(ctx, req)
}

func (c *InstrumentedClient) Model() string {
	if m, ok := c.inner.(interface{ Model() string }); ok {
		return m.Model()
	}
	return ""
}
