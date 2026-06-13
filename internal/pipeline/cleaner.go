package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"media2rag/internal/events"
	"media2rag/internal/llm"
	"media2rag/internal/model"
)

type cleanPartJob struct {
	index int
	text  string
}

func (p *Pipeline) preClean(ctx context.Context, text string, emitter events.EventEmitter) (string, error) {
	if restored, ok := p.tryRestoreCleaned(emitter); ok {
		return restored, nil
	}

	emitter.Emit(model.Event{Type: EventPreClean})

	if len(text) <= maxCompressLen {
		cleaned, err := p.cleanSinglePart(ctx, text)
		if err != nil {
			return "", err
		}
		return p.saveCleanedResult(cleaned)
	}

	return p.cleanParallel(ctx, text, emitter)
}

func (p *Pipeline) tryRestoreCleaned(emitter events.EventEmitter) (string, bool) {
	if p.status != nil && p.status.Stage != "" && p.status.Stage != StageExtracted {
		if data, err := os.ReadFile(filepath.Join(p.outputDir, "intermediate", "cleaned.md")); err == nil {
			emitter.Emit(model.Event{Type: EventCheckpointRestore, Data: map[string]string{"stage": "cleaned"}})
			return string(data), true
		}
	}

	if p.checkpointDir != "" {
		if data, err := loadCheckpointFile(p.checkpointDir, "cleaned.md"); err == nil {
			emitter.Emit(model.Event{Type: EventCheckpointRestore, Data: map[string]string{"stage": "cleaned"}})
			return string(data), true
		}
	}

	return "", false
}

func (p *Pipeline) cleanParallel(ctx context.Context, text string, emitter events.EventEmitter) (string, error) {
	parts := splitParagraphParts(text, maxCompressLen)
	cleanedParts := make([]string, len(parts))

	pool := &WorkerPool[cleanPartJob]{
		NumWorkers: p.config.MaxConcurrency,
		ProcessFn: func(ctx context.Context, job cleanPartJob) error {
			emitter.Emit(model.Event{Type: EventCleaningPart, Data: map[string]int{"part": job.index + 1, "total": len(parts)}})
			result, err := p.cleanSinglePart(ctx, job.text)
			if err != nil {
				return fmt.Errorf("clean part %d/%d: %w", job.index+1, len(parts), err)
			}
			cleanedParts[job.index] = result
			return nil
		},
	}

	jobs := make([]cleanPartJob, len(parts))
	for i, part := range parts {
		jobs[i] = cleanPartJob{index: i, text: part}
	}

	if err := pool.Run(ctx, jobs); err != nil {
		return "", err
	}

	cleaned := strings.Join(cleanedParts, "\n\n")
	return p.saveCleanedResult(cleaned)
}

func (p *Pipeline) saveCleanedResult(cleaned string) (string, error) {
	if p.checkpointDir != "" {
		_ = saveCheckpointFile(p.checkpointDir, "cleaned.md", []byte(cleaned))
	}

	if p.status != nil {
		p.status.SetStage(StageCleaned)
	}

	return cleaned, nil
}

func (p *Pipeline) cleanSinglePart(ctx context.Context, text string) (string, error) {
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
		callCtx = llm.WithStage(callCtx, "pre_clean")
		callCtx = llm.WithSource(callCtx, p.source)
		resp, err := p.llmClient.Chat(callCtx, model.ChatRequest{
			Messages: []model.Message{
				{Role: "system", Content: cleaningPrompt},
				{Role: "user", Content: text},
			},
		})
		cancel()
		if err != nil {
			lastErr = err
			continue
		}

		content := strings.TrimSpace(resp.Message.Content)
		if content == "" {
			lastErr = fmt.Errorf("llm returned empty response")
			continue
		}
		return content, nil
	}
	return "", fmt.Errorf("all %d attempts failed: %w", maxRetries+1, lastErr)
}
