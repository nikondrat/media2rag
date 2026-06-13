package pipeline

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"media2rag/internal/events"
	"media2rag/internal/model"
)

func (p *Pipeline) contextualEnrich(ctx context.Context, results []ChunkResult, docContext string, emitter events.EventEmitter) error {
	docSummary := docContext
	if docSummary == "" {
		docSummary = buildDocSummary(results)
	}

	var mu sync.Mutex

	pool := &WorkerPool[int]{
		NumWorkers: p.config.MaxConcurrency,
		ProcessFn: func(ctx context.Context, index int) error {
			r := &results[index]
			if r.Content == "" || r.Summary == "" {
				return nil
			}

			chunkContext, err := p.enrichSingle(ctx, docSummary, r)
			if err != nil {
				return err
			}

			mu.Lock()
			r.Context = chunkContext
			p.writeResultJSON(*r)
			mu.Unlock()

			emitter.Emit(model.Event{Type: EventContextEnrichDone, Data: map[string]int{"chunk": index + 1}})
			return nil
		},
	}

	indices := make([]int, len(results))
	for i := range indices {
		indices[i] = i
	}

	return pool.Run(ctx, indices)
}

func buildDocSummary(results []ChunkResult) string {
	var topics []string
	for _, r := range results {
		if r.Topic != "" {
			topics = append(topics, r.Topic)
		}
	}
	if len(topics) == 0 {
		return ""
	}

	unique := map[string]bool{}
	var deduped []string
	for _, t := range topics {
		if !unique[t] {
			unique[t] = true
			deduped = append(deduped, t)
		}
	}
	return "Document covering: " + strings.Join(deduped[:min(5, len(deduped))], ", ")
}

func (p *Pipeline) enrichSingle(ctx context.Context, docSummary string, r *ChunkResult) (string, error) {
	content := r.Content
	if len(content) > 1000 {
		content = content[:1000]
	}

	userMsg := fmt.Sprintf("Document summary: %s\n\nChunk content:\n%s", docSummary, content)
	raw, err := p.retryLLMCall(ctx, "context_enrich", func(callCtx context.Context) (string, error) {
		resp, err := p.llmClient.Chat(callCtx, model.ChatRequest{
			Messages: []model.Message{
				{Role: "system", Content: contextEnrichPrompt},
				{Role: "user", Content: userMsg},
			},
		})
		if err != nil {
			return "", err
		}
		return resp.Message.Content, nil
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(raw), nil
}
