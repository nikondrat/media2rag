package llm

import (
	"context"

	"media2rag/internal/model"
)

type LLMClient interface {
	Chat(ctx context.Context, req model.ChatRequest) (*model.ChatResponse, error)
	StreamChat(ctx context.Context, req model.ChatRequest) (<-chan model.StreamDelta, error)
	Embed(ctx context.Context, text string) ([]float32, error)
}
