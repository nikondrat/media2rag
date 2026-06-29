package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"media2rag/internal/events"
	"media2rag/internal/llm"
	"media2rag/internal/model"
)

const chunkPrompt = `Analyze the following text and extract structured information. Preserve the original language of the text.

1. type: Classify the chunk into exactly one knowledge ontology type from:
   - idea: A concept, notion, or thought
   - framework: A structured approach or methodology
   - principle: A fundamental truth or rule
   - example: A concrete instance or illustration
   - case_study: A detailed examination of a real-world scenario
   - tool: A practical instrument, technique, or resource
   - warning: A cautionary note or risk
   - action_step: A specific actionable instruction
   - quote: A notable direct quotation
   - question: An open question or point of inquiry
   - personal_note: A subjective reflection or note

2. topic: The single primary topic (3-8 words)

3. summary: A 2-3 sentence summary of the key content in the original language

4. key_points: Comma-separated list of the most important points (max 6)

5. source_quote: A notable direct quote from the text, if any (otherwise leave empty)

6. my_takeaway: The single most important personal takeaway or lesson in the original language

7. confidence: Your confidence in this extraction as a float between 0.0 and 1.0

8. applicability: How this content can be applied in practice (1-2 sentences) in the original language

Return in this exact format:
type: <type>
topic: <topic>
summary: <summary>
key_points: <point1, point2, ...>
source_quote: <quote>
my_takeaway: <takeaway>
confidence: <0.0-1.0>
applicability: <applicability>`

const retryPromptSuffix = `Your previous response was invalid. Common mistakes:
- Returning the template format instead of actual values (e.g., "<type>" instead of "idea")
- Leaving fields empty
- Using a different language than the source text
- Not following the exact format

Please analyze the text again and return properly filled values in the exact format specified.`

type chunkJob struct {
	index int
	text  string
}

func intPtr(v int) *int       { return &v }
func float64Ptr(v float64) *float64 { return &v }

func (p *Pipeline) processChunks(ctx context.Context, chunks []string, emitter events.EventEmitter) ([]ChunkResult, error) {
	results := make([]ChunkResult, len(chunks))
	for i := range results {
		results[i].Index = i
	}

	p.loadCheckpointResults(results, emitter)

	var mu sync.Mutex
	total := len(chunks)
	emitter.Emit(model.Event{Type: EventProcessingStart, Data: map[string]int{"total": total}})

	pool := &WorkerPool[chunkJob]{
		NumWorkers: p.config.MaxConcurrency,
		ProcessFn: func(ctx context.Context, job chunkJob) error {
			emitter.Emit(model.Event{Type: EventProcessingChunk, Data: map[string]int{"chunk": job.index + 1, "total": total}})

			if p.status != nil {
				p.status.ChunkStarted(job.index, "process")
			}

			r, err := p.processSingle(ctx, job.text, job.index, emitter)
			if err != nil {
				if p.status != nil {
					p.status.ChunkFailed(job.index, err.Error())
				}
				return fmt.Errorf("chunk %d: %w", job.index+1, err)
			}
			r.Index = job.index
			r.Content = job.text

			mu.Lock()
			results[job.index] = r
			p.writeResultJSON(r)
			mu.Unlock()

			if p.status != nil {
				p.status.ChunkDone(job.index)
			}
			emitter.Emit(model.Event{Type: EventProcessingChunkDone, Data: map[string]int{"chunk": job.index + 1}})
			return nil
		},
	}

	var jobs []chunkJob
	for i, chunk := range chunks {
		if results[i].Summary != "" || results[i].Type != "" {
			continue
		}
		jobs = append(jobs, chunkJob{index: i, text: chunk})
	}

	if err := pool.Run(ctx, jobs); err != nil {
		return nil, err
	}
	return results, nil
}

func (p *Pipeline) loadCheckpointResults(results []ChunkResult, emitter events.EventEmitter) {
	for i := range results {
		if p.status != nil && p.status.IsChunkDone(i) {
			resultPath := filepath.Join(p.outputDir, "results", fmt.Sprintf("result_%03d.json", i+1))
			if data, err := os.ReadFile(resultPath); err == nil {
				var r ChunkResult
				if err := json.Unmarshal(data, &r); err == nil {
					results[i] = r
					emitter.Emit(model.Event{Type: EventCheckpointRestore, Data: map[string]string{"stage": fmt.Sprintf("chunk %d", i+1)}})
					continue
				}
			}
		}

		if p.outputDir != "" {
			resultPath := filepath.Join(p.outputDir, "results", fmt.Sprintf("result_%03d.json", i+1))
			if data, err := os.ReadFile(resultPath); err == nil {
				var r ChunkResult
				if err := json.Unmarshal(data, &r); err == nil && r.Summary != "" {
					results[i] = r
					if p.status != nil {
						p.status.ChunkDone(i)
					}
					emitter.Emit(model.Event{Type: EventCheckpointRestore, Data: map[string]string{"stage": fmt.Sprintf("chunk %d", i+1)}})
					continue
				}
			}
		}

		if data, err := loadResultFromCheckpoint(p.checkpointDir, i); err == nil && data.Index == i {
			results[i] = data
			p.writeResultJSON(data)
			if p.status != nil {
				p.status.ChunkDone(i)
			}
			emitter.Emit(model.Event{Type: EventCheckpointRestore, Data: map[string]string{"stage": fmt.Sprintf("chunk %d", i+1)}})
		}
	}
}

func (p *Pipeline) processSingle(ctx context.Context, text string, chunkIndex int, emitter events.EventEmitter) (ChunkResult, error) {
	const maxRetries = 2
	systemPrompt := chunkPrompt

	for attempt := 0; attempt <= maxRetries; attempt++ {
		callCtx, cancel := p.timeoutCtx(ctx)
		callCtx = llm.WithStage(callCtx, "process")
		callCtx = llm.WithChunkIndex(callCtx, chunkIndex)
		callCtx = llm.WithRetryAttempt(callCtx, attempt)
		start := time.Now()
		resp, err := p.llmClient.Chat(callCtx, model.ChatRequest{
			Messages: []model.Message{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: text},
			},
			MaxTokens:        intPtr(p.config.MaxTokens),
			FrequencyPenalty: float64Ptr(p.config.FrequencyPenalty),
			PresencePenalty:  float64Ptr(p.config.PresencePenalty),
		})
		_ = start
		cancel()

		if err != nil {
			return ChunkResult{}, err
		}

		if detectRepetition(resp.Message.Content) {
			if attempt < maxRetries {
				emitter.Emit(model.Event{
					Type: EventProcessingRetry,
					Data: map[string]interface{}{
						"chunk":   chunkIndex + 1,
						"attempt": attempt + 1,
						"error":   "repetition detected",
					},
				})
				systemPrompt = chunkPrompt + "\n\n" +
					"CRITICAL: Your previous response contained repetitive hallucinated text. " +
					"Generate a completely NEW response. Do NOT repeat words or phrases."
				continue
			}
			return ChunkResult{}, fmt.Errorf("repetition detected after retries")
		}

		parsed := parsePromptResult(resp.Message.Content)

		if err := validateChunkResult(parsed); err != nil {
			if attempt < maxRetries {
				emitter.Emit(model.Event{
					Type: EventProcessingRetry,
					Data: map[string]interface{}{
						"chunk":   chunkIndex + 1,
						"attempt": attempt + 1,
						"error":   err.Error(),
					},
				})
				systemPrompt = chunkPrompt + "\n\n" + retryPromptSuffix + "\n\nPrevious invalid response:\n" + resp.Message.Content
				continue
			}
			return ChunkResult{}, fmt.Errorf("chunk validation failed after %d retries: %w", maxRetries, err)
		}

		return parsed, nil
	}

	return ChunkResult{}, fmt.Errorf("unexpected: all retries exhausted")
}
