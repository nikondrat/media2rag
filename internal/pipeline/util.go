package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"media2rag/internal/llm"
)

func (p *Pipeline) retryLLMCall(ctx context.Context, stage string, fn func(context.Context) (string, error)) (string, error) {
	const maxRetries = 2
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * 2 * time.Second
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
		}

		callCtx, cancel := p.timeoutCtx(ctx)
		callCtx = llm.WithStage(callCtx, stage)
		callCtx = llm.WithRetryAttempt(callCtx, attempt)

		result, err := fn(callCtx)
		cancel()
		if err != nil {
			lastErr = err
			continue
		}
		return result, nil
	}

	return "", fmt.Errorf("%s failed after %d retries: %w", stage, maxRetries, lastErr)
}

func (p *Pipeline) timeoutCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	if p.config.LLMTimeout > 0 {
		return context.WithTimeout(ctx, p.config.LLMTimeout)
	}
	return ctx, func() {}
}

func (p *Pipeline) writeResultJSON(result ChunkResult) {
	if p.outputDir == "" {
		return
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return
	}
	name := fmt.Sprintf("result_%03d.json", result.Index+1)
	_ = os.WriteFile(filepath.Join(p.outputDir, "results", name), data, 0644)
}

const maxCompressLen = 28000

func splitParagraphParts(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var parts []string
	for len(text) > 0 {
		if len(text) <= maxLen {
			parts = append(parts, text)
			break
		}

		cut := text[:maxLen]
		lastPara := strings.LastIndex(cut, "\n\n")
		if lastPara > maxLen/2 {
			parts = append(parts, text[:lastPara])
			text = text[lastPara:]
			continue
		}

		lastNewline := strings.LastIndex(cut, "\n")
		if lastNewline > maxLen/2 {
			parts = append(parts, text[:lastNewline])
			text = text[lastNewline:]
			continue
		}

		for len(cut) > 0 && !utf8.ValidString(cut) {
			cut = cut[:len(cut)-1]
		}
		if len(cut) == 0 {
			cut = text[:maxLen]
		}
		parts = append(parts, cut)
		text = text[len(cut):]
	}
	return parts
}

func loadCheckpointFile(dir, name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(dir, name))
}

func saveCheckpointFile(dir, name string, data []byte) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create checkpoint dir: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, name), data, 0644)
}
