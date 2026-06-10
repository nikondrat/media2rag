package llm

import (
	"context"

	"media2rag/internal/model"
)

type ctxKey string

const (
	ctxKeyStage       ctxKey = "llm_stage"
	ctxKeyChunkIndex  ctxKey = "llm_chunk_index"
	ctxKeySource      ctxKey = "llm_source"
	ctxKeyRetryAttempt ctxKey = "llm_retry_attempt"
)

func WithStage(ctx context.Context, stage string) context.Context {
	return context.WithValue(ctx, ctxKeyStage, stage)
}

func WithChunkIndex(ctx context.Context, index int) context.Context {
	return context.WithValue(ctx, ctxKeyChunkIndex, index)
}

func WithSource(ctx context.Context, source string) context.Context {
	return context.WithValue(ctx, ctxKeySource, source)
}

func WithRetryAttempt(ctx context.Context, attempt int) context.Context {
	return context.WithValue(ctx, ctxKeyRetryAttempt, attempt)
}

func stageFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyStage).(string); ok {
		return v
	}
	return ""
}

func chunkFromCtx(ctx context.Context) int {
	if v, ok := ctx.Value(ctxKeyChunkIndex).(int); ok {
		return v
	}
	return 0
}

func sourceFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeySource).(string); ok {
		return v
	}
	return ""
}

func retryFromCtx(ctx context.Context) int {
	if v, ok := ctx.Value(ctxKeyRetryAttempt).(int); ok {
		return v
	}
	return 0
}

func promptChars(messages []model.Message) int {
	total := 0
	for _, m := range messages {
		total += len(m.Content)
	}
	return total
}
